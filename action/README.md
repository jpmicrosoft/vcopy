# vcopy GitHub Action

A reusable GitHub Action that copies repositories between GitHub organizations (Cloud and Enterprise) with 5-layer integrity verification. Wraps the [vcopy CLI tool](../README.md).

## How It Works

1. The action downloads a pre-built `vcopy` binary from the action repository's GitHub Releases
2. Maps your workflow inputs to CLI flags
3. Runs the copy and/or verification
4. Reports the verification status as an output

## Quick Start

```yaml
- uses: your-org/vcopy@v1
  with:
    source-repo: source-org/my-repo
    target-org: target-org
    source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
    target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

## Setup in Your Organization

Since this is a private repository, you'll need to clone or fork it into your own GitHub organization to use the action.

### Step 1: Clone the repo to your org

```bash
# Clone vcopy to your org
git clone https://github.com/original-owner/vcopy.git
cd vcopy
git remote set-url origin https://github.com/your-org/vcopy.git
git push --mirror
```

### Step 2: Create a release

The action downloads pre-built binaries from your repo's GitHub Releases. Push a version tag to trigger the release workflow:

```bash
git tag v1.0.0
git push origin v1.0.0
```

This triggers `.github/workflows/release.yml`, which:
- Builds binaries for Linux, macOS, and Windows (amd64 + arm64)
- Creates a GitHub Release with all binaries attached
- Updates the floating `v1` tag so `uses: your-org/vcopy@v1` always gets the latest

### Step 3: Use in your workflows

```yaml
- uses: your-org/vcopy@v1
  with:
    source-repo: source-org/my-repo
    target-org: target-org
    target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

## Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `source-repo` | ✅ | | Source repository in `owner/repo` format |
| `target-org` | ✅ | | Target organization or user account |
| `target-token` | ✅ | | Personal Access Token for the target instance |
| `source-token` | | | PAT for the source instance (not needed for public repos) |
| `source-host` | | `github.com` | Source GitHub hostname |
| `target-host` | | `github.com` | Target GitHub hostname (e.g., `github.mycompany.com`) |
| `target-name` | | same as source | Target repository name |
| `public` | | `false` | Source repo is public — skip source auth |
| `lfs` | | `false` | Include Git LFS objects |
| `force` | | `false` | Destructive mirror push (overwrites everything in target) |
| `code-only` | | `false` | Copy only branches/commits (no tags, releases, or metadata) |
| `issues` | | `false` | Copy issues |
| `pull-requests` | | `false` | Copy pull requests (as issues) |
| `wiki` | | `false` | Copy the wiki |
| `all-metadata` | | `false` | Copy all metadata (issues, PRs, wiki) |
| `verify-only` | | `false` | Only verify — do not copy |
| `skip-verify` | | `false` | Skip verification after copy |
| `quick-verify` | | `false` | Quick verification (refs + tree hashes only) |
| `since` | | | Incremental verify: only objects after this SHA or date |
| `report` | | | File path for JSON verification report |
| `sign` | | | GPG key ID to sign the report |
| `verbose` | | `false` | Show detailed output |
| `version` | | `latest` | vcopy release version (e.g., `v1.0.0`) |
| `upload-report` | | `false` | Upload the verification report as a workflow artifact |
| `artifact-name` | | `vcopy-verification-report` | Name for the uploaded artifact (used with `upload-report`) |
| `no-workflows` | | `false` | Exclude GitHub Actions workflows and custom actions (`.github/workflows/`, `.github/actions/`) from the target |
| `no-copilot` | | `false` | Exclude Copilot instructions/skills from the target |
| `exclude` | | | Comma-separated paths to exclude from the target |

> **Note**: `CODEOWNERS` files (`CODEOWNERS`, `.github/CODEOWNERS`, `docs/CODEOWNERS`) are **always removed** on every copy because they reference source org teams/users that won't exist in the target.

## Outputs

| Output | Description |
|--------|-------------|
| `verification-status` | `pass`, `fail`, or `skipped` |
| `report-path` | Path to the JSON report file (if `report` was set) |

## Example Workflows

### Basic copy (Cloud to Cloud)

```yaml
name: Copy Repository
on:
  workflow_dispatch:

jobs:
  copy:
    runs-on: ubuntu-latest
    steps:
      - uses: your-org/vcopy@v1
        with:
          source-repo: source-org/my-repo
          target-org: target-org
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Copy public repo (no source token needed)

```yaml
      - uses: your-org/vcopy@v1
        with:
          source-repo: kubernetes/kubernetes
          target-org: my-org
          public: true
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Code only (no tags or releases)

```yaml
      - uses: your-org/vcopy@v1
        with:
          source-repo: source-org/my-repo
          target-org: target-org
          code-only: true
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Exclude workflows and Copilot config

```yaml
      - uses: your-org/vcopy@v1
        with:
          source-repo: source-org/my-repo
          target-org: target-org
          no-workflows: true
          no-copilot: true
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Exclude custom paths

```yaml
      - uses: your-org/vcopy@v1
        with:
          source-repo: source-org/my-repo
          target-org: target-org
          no-workflows: true
          exclude: vendor,docs/internal
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Copy to GitHub Enterprise Server

```yaml
      - uses: your-org/vcopy@v1
        with:
          source-repo: cloud-org/my-repo
          target-org: enterprise-org
          target-host: github.mycompany.com
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.ENTERPRISE_TOKEN }}
```

### Force overwrite existing target

```yaml
      - uses: your-org/vcopy@v1
        with:
          source-repo: source-org/my-repo
          target-org: target-org
          force: true
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

> ⚠️ **Warning**: `force: true` performs a destructive mirror push **without confirmation** (the action runs in non-interactive mode). All branches, tags, and releases in the target that don't exist in the source will be permanently deleted.

### Verify only (no copy)

```yaml
      - uses: your-org/vcopy@v1
        id: verify
        with:
          source-repo: source-org/my-repo
          target-org: target-org
          verify-only: true
          report: verification-report.json
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}

      - name: Check result
        run: echo "Verification: ${{ steps.verify.outputs.verification-status }}"
```

### Copy with all metadata and audit report

```yaml
      - uses: your-org/vcopy@v1
        with:
          source-repo: source-org/my-repo
          target-org: target-org
          all-metadata: true
          report: audit.json
          upload-report: true
          verbose: true
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

The report is automatically uploaded as a workflow artifact named `vcopy-verification-report`. You can customize the name:

```yaml
          upload-report: true
          artifact-name: my-repo-audit-2025
```

### Scheduled sync

```yaml
name: Nightly Repository Sync
on:
  schedule:
    - cron: '0 2 * * *'  # 2 AM UTC daily

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: your-org/vcopy@v1
        with:
          source-repo: upstream-org/shared-lib
          target-org: my-org
          quick-verify: true
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

## Token Permissions

Tokens need the `repo` scope (full control of private repositories) on both source and target instances. For public source repos, use `public: true` and omit `source-token`.

## Supported Runners

| Runner OS | Architectures |
|-----------|--------------|
| `ubuntu-latest` | x64, arm64 |
| `macos-latest` | x64, arm64 |
| `windows-latest` | x64, arm64 |

## Troubleshooting

### "Binary not found in release"
Ensure you've created a release by pushing a version tag (`git tag v1.0.0 && git push origin v1.0.0`). The release workflow builds binaries for all platforms.

### "Failed to fetch release"
The action uses `GITHUB_TOKEN` to download binaries. For private repos, ensure the workflow has `contents: read` permission (default for most workflows).

### Authentication failures
- Check that tokens have the `repo` scope
- For Enterprise, ensure the hostname is correct (`target-host: github.mycompany.com`)
- For public repos, set `public: true` to skip source authentication
