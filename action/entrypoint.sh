#!/usr/bin/env bash
set -euo pipefail

# vcopy GitHub Action entrypoint
# Downloads the vcopy binary and runs it with inputs from the workflow.

# ──────────────────────────────────────────────
# 1. Detect runner OS and architecture
# ──────────────────────────────────────────────
case "${RUNNER_OS}" in
  Linux)  OS="linux" ;;
  macOS)  OS="darwin" ;;
  Windows) OS="windows" ;;
  *) echo "::error::Unsupported runner OS: ${RUNNER_OS}"; exit 1 ;;
esac

case "${RUNNER_ARCH}" in
  X64)   ARCH="amd64" ;;
  ARM64) ARCH="arm64" ;;
  *) echo "::error::Unsupported runner architecture: ${RUNNER_ARCH}"; exit 1 ;;
esac

EXT=""
if [ "${OS}" = "windows" ]; then
  EXT=".exe"
fi

BINARY_NAME="vcopy-${OS}-${ARCH}${EXT}"
echo "Platform: ${OS}/${ARCH} → ${BINARY_NAME}"

# ──────────────────────────────────────────────
# 2. Validate inputs
# ──────────────────────────────────────────────
VERSION="${INPUT_VERSION}"
if [ "${VERSION}" != "latest" ]; then
  if ! echo "${VERSION}" | grep -qE '^v[0-9]+(\.[0-9]+)*(-[a-zA-Z0-9._]+)?$'; then
    echo "::error::Invalid version format '${VERSION}'. Expected 'latest' or semver like 'v1.0.0'."
    exit 1
  fi
fi

# VCOPY_ACTION_REPO is set by action.yml via github.action_repository
ACTION_REPO="${VCOPY_ACTION_REPO:-${GITHUB_REPOSITORY}}"
echo "Action repository: ${ACTION_REPO}"

# ──────────────────────────────────────────────
# 3. Download vcopy binary from GitHub Releases
# ──────────────────────────────────────────────
INSTALL_DIR="${RUNNER_TEMP}/vcopy"
mkdir -p "${INSTALL_DIR}"

if [ "${VERSION}" = "latest" ]; then
  RELEASE_URL="https://api.github.com/repos/${ACTION_REPO}/releases/latest"
else
  RELEASE_URL="https://api.github.com/repos/${ACTION_REPO}/releases/tags/${VERSION}"
fi

echo "Fetching release info from: ${RELEASE_URL}"

# Try authenticated first, fall back to unauthenticated (public action repos)
fetch_release() {
  if [ -n "${GH_TOKEN}" ]; then
    curl -sSfL -H "Authorization: token ${GH_TOKEN}" -H "Accept: application/vnd.github+json" "$1" 2>/dev/null && return 0
  fi
  curl -sSfL -H "Accept: application/vnd.github+json" "$1" 2>/dev/null && return 0
  return 1
}

RELEASE_JSON=$(fetch_release "${RELEASE_URL}") || {
  echo "::error::Failed to fetch release. Ensure a release exists and the token has access."
  exit 1
}

# Find the asset download URL for our platform
ASSET_URL=$(echo "${RELEASE_JSON}" | grep -o "\"browser_download_url\":[[:space:]]*\"[^\"]*${BINARY_NAME}\"" | head -1 | cut -d'"' -f4)

if [ -z "${ASSET_URL}" ]; then
  echo "::error::Binary '${BINARY_NAME}' not found in release. Available assets:"
  echo "${RELEASE_JSON}" | grep -o '"name":"[^"]*"' || true
  exit 1
fi

echo "Downloading: ${ASSET_URL}"

# For private repos, use the API URL with Accept header for the asset
ASSET_ID=$(echo "${RELEASE_JSON}" | grep -B5 "${BINARY_NAME}" | grep -o '"id":[[:space:]]*[0-9]*' | head -1 | grep -o '[0-9]*')
ASSET_API_URL="https://api.github.com/repos/${ACTION_REPO}/releases/assets/${ASSET_ID}"

download_asset() {
  if [ -n "${GH_TOKEN}" ]; then
    curl -sSfL -H "Authorization: token ${GH_TOKEN}" -H "Accept: application/octet-stream" "$1" -o "$2" 2>/dev/null && return 0
  fi
  curl -sSfL -H "Accept: application/octet-stream" "$1" -o "$2" 2>/dev/null && return 0
  # Last resort: try browser URL directly (public repos)
  curl -sSfL -L "${ASSET_URL}" -o "$2" 2>/dev/null && return 0
  return 1
}

download_asset "${ASSET_API_URL}" "${INSTALL_DIR}/vcopy${EXT}" || {
  echo "::error::Failed to download vcopy binary."
  exit 1
}

# ──────────────────────────────────────────────
# 4. Verify binary integrity via checksums
# ──────────────────────────────────────────────
CHECKSUM_ID=$(echo "${RELEASE_JSON}" | grep -B5 "checksums.txt" | grep -o '"id":[[:space:]]*[0-9]*' | head -1 | grep -o '[0-9]*' || true)

if [ -n "${CHECKSUM_ID}" ]; then
  CHECKSUM_API_URL="https://api.github.com/repos/${ACTION_REPO}/releases/assets/${CHECKSUM_ID}"
  download_asset "${CHECKSUM_API_URL}" "${INSTALL_DIR}/checksums.txt" || {
    echo "::warning::Could not download checksums.txt — skipping integrity check."
  }

  if [ -f "${INSTALL_DIR}/checksums.txt" ]; then
    EXPECTED=$(grep "${BINARY_NAME}" "${INSTALL_DIR}/checksums.txt" | awk '{print $1}')
    if [ "${OS}" = "windows" ]; then
      ACTUAL=$(powershell -Command "(Get-FileHash -Algorithm SHA256 '${INSTALL_DIR}/vcopy${EXT}').Hash.ToLower()")
    else
      ACTUAL=$(sha256sum "${INSTALL_DIR}/vcopy${EXT}" | awk '{print $1}')
    fi
    if [ "${EXPECTED}" != "${ACTUAL}" ]; then
      echo "::error::Checksum mismatch! Expected ${EXPECTED}, got ${ACTUAL}. Binary may be corrupted or tampered with."
      rm -f "${INSTALL_DIR}/vcopy${EXT}"
      exit 1
    fi
    echo "✓ Binary checksum verified: ${ACTUAL}"
  fi
else
  echo "::warning::No checksums.txt found in release — skipping integrity check."
fi

chmod +x "${INSTALL_DIR}/vcopy${EXT}" || {
  echo "::error::Failed to make binary executable."
  exit 1
}
echo "vcopy installed to: ${INSTALL_DIR}/vcopy${EXT}"

VCOPY="${INSTALL_DIR}/vcopy${EXT}"

# ──────────────────────────────────────────────
# 5. Build CLI arguments from action inputs
# ──────────────────────────────────────────────
ARGS=()

MODE="${INPUT_MODE:-single}"

if [ -z "${INPUT_TARGET_ORG}" ]; then
  echo "::error::target-org is required and cannot be empty"
  exit 1
fi

if [ "${MODE}" = "batch" ]; then
  # Batch mode: vcopy batch <source-org> <target-org> --search <filter>
  if [ -z "${INPUT_SOURCE_ORG}" ]; then
    echo "::error::source-org is required when mode=batch"
    exit 1
  fi
  if [ -z "${INPUT_SEARCH}" ]; then
    echo "::error::search is required when mode=batch"
    exit 1
  fi

  ARGS+=("batch" "${INPUT_SOURCE_ORG}" "${INPUT_TARGET_ORG}")
  ARGS+=("--search" "${INPUT_SEARCH}")
  [ -n "${INPUT_PREFIX}" ] && ARGS+=("--prefix" "${INPUT_PREFIX}")
  [ -n "${INPUT_SUFFIX}" ] && ARGS+=("--suffix" "${INPUT_SUFFIX}")
  [ "${INPUT_SKIP_EXISTING}" = "true" ] && ARGS+=("--skip-existing")
  [ "${INPUT_SYNC}" = "true" ] && ARGS+=("--sync")
  [ -n "${INPUT_REPORT}" ] && ARGS+=("--report" "${INPUT_REPORT}")
  [ "${INPUT_PER_REPO_REPORT}" = "true" ] && ARGS+=("--per-repo-report")
else
  # Single mode: vcopy <source-repo> <target-org>
  if [ -z "${INPUT_SOURCE_REPO}" ]; then
    echo "::error::source-repo is required when mode=single"
    exit 1
  fi
  ARGS+=("${INPUT_SOURCE_REPO}" "${INPUT_TARGET_ORG}")
  [ -n "${INPUT_TARGET_NAME}" ] && ARGS+=("--target-name" "${INPUT_TARGET_NAME}")
  [ "${INPUT_FORCE}" = "true" ] && ARGS+=("--force")
  [ "${INPUT_ISSUES}" = "true" ] && ARGS+=("--issues")
  [ "${INPUT_PULL_REQUESTS}" = "true" ] && ARGS+=("--pull-requests")
  [ "${INPUT_WIKI}" = "true" ] && ARGS+=("--wiki")
  [ "${INPUT_ALL_METADATA}" = "true" ] && ARGS+=("--all-metadata")
  [ "${INPUT_VERIFY_ONLY}" = "true" ] && ARGS+=("--verify-only")
  [ -n "${INPUT_SINCE}" ] && ARGS+=("--since" "${INPUT_SINCE}")
  [ -n "${INPUT_REPORT}" ] && ARGS+=("--report" "${INPUT_REPORT}")
  [ -n "${INPUT_SIGN}" ] && ARGS+=("--sign" "${INPUT_SIGN}")
fi

# Auth — always use PAT mode in CI
ARGS+=("--auth-method" "pat")

# Always non-interactive in CI
ARGS+=("--non-interactive")

# Shared flags (apply to both single and batch mode)
[ -n "${INPUT_SOURCE_HOST}" ] && [ "${INPUT_SOURCE_HOST}" != "github.com" ] && ARGS+=("--source-host" "${INPUT_SOURCE_HOST}")
[ -n "${INPUT_TARGET_HOST}" ] && [ "${INPUT_TARGET_HOST}" != "github.com" ] && ARGS+=("--target-host" "${INPUT_TARGET_HOST}")
[ -n "${INPUT_SOURCE_TOKEN}" ] && ARGS+=("--source-token" "${INPUT_SOURCE_TOKEN}")
[ -n "${INPUT_TARGET_TOKEN}" ] && ARGS+=("--target-token" "${INPUT_TARGET_TOKEN}")
# Support both public-source (new) and public (deprecated) inputs
if [ "${INPUT_PUBLIC_SOURCE}" = "true" ] || [ "${INPUT_PUBLIC}" = "true" ]; then
  ARGS+=("--public-source")
fi
[ -n "${INPUT_VISIBILITY}" ] && ARGS+=("--visibility" "${INPUT_VISIBILITY}")
[ "${INPUT_LFS}" = "true" ] && ARGS+=("--lfs")
[ "${INPUT_CODE_ONLY}" = "true" ] && ARGS+=("--code-only")
[ "${INPUT_SKIP_VERIFY}" = "true" ] && ARGS+=("--skip-verify")
[ "${INPUT_QUICK_VERIFY}" = "true" ] && ARGS+=("--quick-verify")
[ "${INPUT_VERBOSE}" = "true" ] && ARGS+=("--verbose")
[ "${INPUT_NO_WORKFLOWS}" = "true" ] && ARGS+=("--no-workflows")
[ "${INPUT_NO_ACTIONS}" = "true" ] && ARGS+=("--no-actions")
[ "${INPUT_NO_COPILOT}" = "true" ] && ARGS+=("--no-copilot")
[ "${INPUT_NO_GITHUB}" = "true" ] && ARGS+=("--no-github")
[ "${INPUT_DRY_RUN}" = "true" ] && ARGS+=("--dry-run")
[ -n "${INPUT_EXCLUDE}" ] && ARGS+=("--exclude" "${INPUT_EXCLUDE}")
[ -n "${INPUT_BATCH_DELAY}" ] && [ "${MODE}" = "batch" ] && ARGS+=("--batch-delay" "${INPUT_BATCH_DELAY}")

# ──────────────────────────────────────────────
# 6. Run vcopy (tokens are masked by GitHub Actions automatically)
# ──────────────────────────────────────────────
echo "::group::vcopy output"
echo "Running: vcopy [args redacted for security]"

EXIT_CODE=0
"${VCOPY}" "${ARGS[@]}" || EXIT_CODE=$?

echo "::endgroup::"

# ──────────────────────────────────────────────
# 7. Set outputs
# ──────────────────────────────────────────────
if [ "${INPUT_SKIP_VERIFY}" = "true" ]; then
  echo "verification_status=skipped" >> "${GITHUB_OUTPUT}"
elif [ ${EXIT_CODE} -eq 0 ]; then
  echo "verification_status=pass" >> "${GITHUB_OUTPUT}"
else
  echo "verification_status=fail" >> "${GITHUB_OUTPUT}"
fi

if [ -n "${INPUT_REPORT}" ]; then
  echo "report_path=${INPUT_REPORT}" >> "${GITHUB_OUTPUT}"
fi

exit ${EXIT_CODE}
