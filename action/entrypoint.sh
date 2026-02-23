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
# 2. Determine which repo to download from
# ──────────────────────────────────────────────
# github.action_repository is the repo where the action lives (supports forks)
ACTION_REPO="${GITHUB_ACTION_REPOSITORY:-${GITHUB_REPOSITORY}}"
echo "Action repository: ${ACTION_REPO}"

# ──────────────────────────────────────────────
# 3. Download vcopy binary from GitHub Releases
# ──────────────────────────────────────────────
VERSION="${INPUT_VERSION}"
INSTALL_DIR="${RUNNER_TEMP}/vcopy"
mkdir -p "${INSTALL_DIR}"

if [ "${VERSION}" = "latest" ]; then
  RELEASE_URL="https://api.github.com/repos/${ACTION_REPO}/releases/latest"
else
  RELEASE_URL="https://api.github.com/repos/${ACTION_REPO}/releases/tags/${VERSION}"
fi

echo "Fetching release info from: ${RELEASE_URL}"

# Use GH_TOKEN for authentication (required for private repos)
RELEASE_JSON=$(curl -sSfL \
  -H "Authorization: token ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  "${RELEASE_URL}") || {
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

# For private repos, we need to use the API URL with Accept header for the asset
ASSET_ID=$(echo "${RELEASE_JSON}" | grep -B5 "${BINARY_NAME}" | grep -o '"id":[[:space:]]*[0-9]*' | head -1 | grep -o '[0-9]*')
ASSET_API_URL="https://api.github.com/repos/${ACTION_REPO}/releases/assets/${ASSET_ID}"

curl -sSfL \
  -H "Authorization: token ${GH_TOKEN}" \
  -H "Accept: application/octet-stream" \
  "${ASSET_API_URL}" \
  -o "${INSTALL_DIR}/vcopy${EXT}" || {
  echo "::error::Failed to download vcopy binary."
  exit 1
}

chmod +x "${INSTALL_DIR}/vcopy${EXT}"
echo "vcopy installed to: ${INSTALL_DIR}/vcopy${EXT}"

VCOPY="${INSTALL_DIR}/vcopy${EXT}"

# ──────────────────────────────────────────────
# 4. Build CLI arguments from action inputs
# ──────────────────────────────────────────────
ARGS=()

# Required positional args
ARGS+=("${INPUT_SOURCE_REPO}" "${INPUT_TARGET_ORG}")

# Auth — always use PAT mode in CI
ARGS+=("--auth-method" "pat")

# Always non-interactive in CI
ARGS+=("--non-interactive")

# String inputs
[ -n "${INPUT_SOURCE_HOST}" ] && [ "${INPUT_SOURCE_HOST}" != "github.com" ] && ARGS+=("--source-host" "${INPUT_SOURCE_HOST}")
[ -n "${INPUT_TARGET_HOST}" ] && [ "${INPUT_TARGET_HOST}" != "github.com" ] && ARGS+=("--target-host" "${INPUT_TARGET_HOST}")
[ -n "${INPUT_TARGET_NAME}" ] && ARGS+=("--target-name" "${INPUT_TARGET_NAME}")
[ -n "${INPUT_SOURCE_TOKEN}" ] && ARGS+=("--source-token" "${INPUT_SOURCE_TOKEN}")
[ -n "${INPUT_TARGET_TOKEN}" ] && ARGS+=("--target-token" "${INPUT_TARGET_TOKEN}")
[ -n "${INPUT_SINCE}" ] && ARGS+=("--since" "${INPUT_SINCE}")
[ -n "${INPUT_REPORT}" ] && ARGS+=("--report" "${INPUT_REPORT}")
[ -n "${INPUT_SIGN}" ] && ARGS+=("--sign" "${INPUT_SIGN}")

# Boolean inputs
[ "${INPUT_PUBLIC}" = "true" ] && ARGS+=("--public")
[ "${INPUT_LFS}" = "true" ] && ARGS+=("--lfs")
[ "${INPUT_FORCE}" = "true" ] && ARGS+=("--force")
[ "${INPUT_CODE_ONLY}" = "true" ] && ARGS+=("--code-only")
[ "${INPUT_ISSUES}" = "true" ] && ARGS+=("--issues")
[ "${INPUT_PULL_REQUESTS}" = "true" ] && ARGS+=("--pull-requests")
[ "${INPUT_WIKI}" = "true" ] && ARGS+=("--wiki")
[ "${INPUT_ALL_METADATA}" = "true" ] && ARGS+=("--all-metadata")
[ "${INPUT_VERIFY_ONLY}" = "true" ] && ARGS+=("--verify-only")
[ "${INPUT_SKIP_VERIFY}" = "true" ] && ARGS+=("--skip-verify")
[ "${INPUT_QUICK_VERIFY}" = "true" ] && ARGS+=("--quick-verify")
[ "${INPUT_VERBOSE}" = "true" ] && ARGS+=("--verbose")

# ──────────────────────────────────────────────
# 5. Run vcopy
# ──────────────────────────────────────────────
echo "::group::vcopy output"
echo "Running: vcopy ${ARGS[*]/%TOKEN*/[REDACTED]}"

EXIT_CODE=0
"${VCOPY}" "${ARGS[@]}" || EXIT_CODE=$?

echo "::endgroup::"

# ──────────────────────────────────────────────
# 6. Set outputs
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
