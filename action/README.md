# vcopy GitHub Action

A reusable GitHub Action that copies repositories between GitHub organizations (Cloud and Enterprise) with 5-layer integrity verification. Wraps the [vcopy CLI tool](../README.md).

## How It Works

1. The action downloads a pre-built `vcopy` binary from the [latest release](https://github.com/jpmicrosoft/vcopy/releases)
2. Maps your workflow inputs to CLI flags
3. Runs the copy and/or verification
4. Reports the verification status as an output

## Quick Start

```yaml
- uses: jpmicrosoft/vcopy@v1
  with:
    source-repo: source-org/my-repo
    target-org: target-org
    source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
    target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

No setup required — the action is publicly available from the GitHub Marketplace.

## Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `source-repo` | | | Source repository in `owner/repo` format. Required for single-repo mode. |
| `target-org` | ✅ | | Target organization or user account |
| `target-token` | ✅ | | Personal Access Token for the target instance |
| `mode` | | `single` | Copy mode: `single` (one repo) or `batch` (multiple repos via search) |
| `source-org` | | | Source organization for batch mode (e.g., `Azure`). Required when `mode=batch`. |
| `search` | | | Repo name filter for batch mode (e.g., `terraform-azurerm-avm-`). Required when `mode=batch`. |
| `prefix` | | | Prefix for target repo names (batch mode) |
| `suffix` | | | Suffix for target repo names (batch mode) |
| `skip-existing` | | `false` | Skip repos already in target org (batch mode, for resuming) |
| `sync` | | `false` | Update existing repos (additive push + incremental release sync) instead of skipping them (batch mode). Mutually exclusive with `skip-existing`. |
| `per-repo-report` | | `false` | Also write individual JSON reports per repo in batch mode (e.g., `report-reponame.json`) |
| `source-token` | | | PAT for the source instance (not needed for public repos) |
| `source-host` | | `github.com` | Source GitHub hostname |
| `target-host` | | `github.com` | Target GitHub hostname (e.g., `github.mycompany.com`) |
| `target-name` | | same as source | Target repository name |
| `public-source` | | `false` | Source repo is public — skip source auth |
| `visibility` | | `private` | Target repo visibility: `private`, `public`, or `internal` |
| `lfs` | | `false` | Include Git LFS objects |
| `force` | | `false` | Destructive mirror push (overwrites everything in target) |
| `code-only` | | `false` | Copy only branches/commits (no tags, releases, or metadata) |
| `issues` | | `false` | Copy issues |
| `pull-requests` | | `false` | Copy pull requests (as issues) |
| `wiki` | | `false` | Copy the wiki |
| `all-metadata` | | `false` | Copy all metadata (issues, PRs, wiki) |
| `verify-only` | | `false` | Only verify — do not copy. Cannot be combined with `skip-verify` |
| `skip-verify` | | `false` | Skip verification after copy. Cannot be combined with `verify-only` |
| `quick-verify` | | `false` | Quick verification (refs + tree hashes only) |
| `since` | | | Incremental verify: only objects after this SHA or date |
| `report` | | | File path for JSON verification report (single mode: per-repo, batch mode: combined) |
| `sign` | | | GPG key ID to sign the report |
| `verbose` | | `false` | Show detailed output |
| `version` | | `latest` | vcopy release version (e.g., `v1.0.0`) |
| `upload-report` | | `false` | Upload the verification report as a workflow artifact |
| `artifact-name` | | `vcopy-verification-report` | Name for the uploaded artifact (used with `upload-report`) |
| `no-workflows` | | `false` | Exclude GitHub Actions workflows (`.github/workflows/`) from the target |
| `no-actions` | | `false` | Exclude GitHub Actions custom actions (`.github/actions/`) from the target |
| `no-copilot` | | `false` | Exclude Copilot instructions/skills from the target |
| `no-github` | | `false` | Exclude entire `.github/` directory (supersedes `no-workflows`, `no-actions`, `no-copilot`) |
| `dry-run` | | `false` | Show what would be done without making changes |
| `exclude` | | | Comma-separated paths to exclude from the target |
| `batch-delay` | | `3s` | Delay between repos in batch mode to avoid secondary rate limits (e.g., `5s`, `0s` to disable) |

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
      - uses: jpmicrosoft/vcopy@v1
        with:
          source-repo: source-org/my-repo
          target-org: target-org
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Copy public repo (no source token needed)

```yaml
      - uses: jpmicrosoft/vcopy@v1
        with:
          source-repo: kubernetes/kubernetes
          target-org: my-org
          public-source: true
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Code only (no tags or releases)

```yaml
      - uses: jpmicrosoft/vcopy@v1
        with:
          source-repo: source-org/my-repo
          target-org: target-org
          code-only: true
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Exclude workflows and Copilot config

```yaml
      - uses: jpmicrosoft/vcopy@v1
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
      - uses: jpmicrosoft/vcopy@v1
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
      - uses: jpmicrosoft/vcopy@v1
        with:
          source-repo: cloud-org/my-repo
          target-org: enterprise-org
          target-host: github.mycompany.com
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.ENTERPRISE_TOKEN }}
```

### Force overwrite existing target

```yaml
      - uses: jpmicrosoft/vcopy@v1
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
      - uses: jpmicrosoft/vcopy@v1
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
      - uses: jpmicrosoft/vcopy@v1
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
      - uses: jpmicrosoft/vcopy@v1
        with:
          source-repo: upstream-org/shared-lib
          target-org: my-org
          quick-verify: true
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Batch copy all Azure Terraform AVM modules

```yaml
name: Batch Copy AVM Modules
on:
  workflow_dispatch:

jobs:
  batch-copy:
    runs-on: ubuntu-latest
    steps:
      - uses: jpmicrosoft/vcopy@v1
        with:
          mode: batch
          source-org: Azure
          target-org: my-org
          search: 'terraform-azurerm-avm-'
          public-source: true
          no-github: true
          skip-verify: true
          skip-existing: true
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Batch copy with prefix/suffix naming

```yaml
      - uses: jpmicrosoft/vcopy@v1
        with:
          mode: batch
          source-org: source-org
          target-org: target-org
          search: 'service-'
          prefix: 'imported-'
          suffix: '-v2'
          skip-existing: true
          source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Batch sync (update existing repos)

```yaml
      - uses: jpmicrosoft/vcopy@v1
        with:
          mode: batch
          source-org: Azure
          target-org: my-org
          search: 'terraform-azurerm-avm-'
          public-source: true
          no-github: true
          sync: true
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

> Sync mode force-updates branches to match the source, adds new tags and releases incrementally (existing tags and releases are preserved), and creates any repos that don't yet exist. Mutually exclusive with `skip-existing`.

### Batch copy with audit report

```yaml
      - uses: jpmicrosoft/vcopy@v1
        with:
          mode: batch
          source-org: Azure
          target-org: my-org
          search: 'terraform-azurerm-avm-'
          public-source: true
          no-github: true
          skip-existing: true
          report: batch-report.json
          per-repo-report: true
          upload-report: true
          artifact-name: avm-batch-audit
          target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

This writes a combined `batch-report.json` with all repos plus individual `batch-report-reponame.json` files for each repo.

## Token Permissions

Tokens need the `repo` scope (full control of private repositories) on both source and target instances. For public source repos, use `public-source: true` and omit `source-token`.

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
- For public repos, set `public-source: true` to skip source authentication

### Verification failures
If a verification check reports FAIL or WARN, see [Verification Failure Troubleshooting](../README.md#verification-failure-troubleshooting) in the main README for per-check cause tables and resolution steps.
