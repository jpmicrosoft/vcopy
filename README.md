# vcopy — Verified GitHub Repository Copy Tool

A CLI tool that copies GitHub repositories between organizations (Cloud and Enterprise) with comprehensive integrity verification.

## Features

- **Verified mirroring** of all branches, tags, and commit history
- **5-layer integrity verification**:
  1. Git object hash verification (every commit, tree, blob)
  2. Branch/tag ref comparison
  3. Root tree hash comparison per branch
  4. GPG/SSH commit signature preservation check
  5. Git bundle SHA-256 checksum verification
- **Flexible authentication**: auto-detects `gh` CLI tokens, falls back to PAT
- **Metadata migration** (optional): issues, pull requests, wiki, releases
- **Audit trail**: colored terminal report + optional JSON output
- **Cross-platform**: single binary for Windows, macOS, Linux

## Installation

### From source

```bash
go install github.com/jaiperez/vcopy/cmd/vcopy@latest
```

### Build from source

```bash
git clone https://github.com/jaiperez/vcopy.git
cd vcopy
go build -o vcopy ./cmd/vcopy
```

### Cross-platform builds

```bash
make build-all
```

Binaries will be in `bin/`.

## Authentication

vcopy supports three authentication methods:

### Auto (default)
Tries `gh` CLI first, falls back to interactive PAT prompt:
```bash
vcopy myorg/myrepo target-org
```

### gh CLI
Uses `gh auth token` exclusively:
```bash
vcopy myorg/myrepo target-org --auth-method gh
```

### PAT (Personal Access Token)
Provide tokens via flags or interactive prompt:
```bash
vcopy myorg/myrepo target-org --auth-method pat --source-token ghp_xxx --target-token ghp_yyy
```

Required token scopes: `repo` (full control of private repositories).

## Usage

### Basic copy (GitHub Cloud to Cloud)

```bash
vcopy myorg/myrepo target-org
```

### Copy to GitHub Enterprise Server

```bash
vcopy myorg/myrepo target-org --target-host github.mycompany.com
```

### Copy between two Enterprise instances

```bash
vcopy myorg/myrepo target-org --source-host github.corp-a.com --target-host github.corp-b.com
```

### Copy with different target repo name

```bash
vcopy myorg/myrepo target-org --target-name new-repo-name
```

### Copy with all metadata

```bash
vcopy myorg/myrepo target-org --all-metadata
```

### Copy with specific metadata

```bash
vcopy myorg/myrepo target-org --issues --releases
```

### Verify only (no copy)

```bash
vcopy myorg/myrepo target-org --verify-only
```

### Generate JSON audit report

```bash
vcopy myorg/myrepo target-org --report audit.json
```

### Dry run

```bash
vcopy myorg/myrepo target-org --dry-run
```

## Public Repositories

Use the `--public` flag when the source repository is publicly accessible. This skips source authentication entirely — you only need credentials for the target.

### How `--public` works

| Feature | With `--public` | Without `--public` |
|---------|----------------|-------------------|
| Git clone/push | ✅ No source token needed | Requires source token |
| Verification (refs, objects, trees, bundle) | ✅ No source token needed | Requires source token |
| Commit signature verification | ✅ No source token needed | Requires source token |
| Metadata copy (issues, PRs, releases) | ⚠️ Works but rate-limited (60 req/hr) | Full rate limit (5,000 req/hr) |
| Wiki copy | ✅ No source token needed | Requires source token |

### Examples

Copy a public repo (no source auth needed):
```bash
vcopy golang/go my-org --public
```

Copy a public repo with metadata (rate-limited for large repos):
```bash
vcopy golang/go my-org --public --all-metadata
```

Copy a public repo to an Enterprise instance:
```bash
vcopy kubernetes/kubernetes my-org --public --target-host github.mycompany.com
```

### Rate limiting with public repos

When copying metadata from public repos without a source token, the GitHub API allows only **60 requests per hour** (unauthenticated). This is usually fine for:
- Repos with fewer than ~50 issues/PRs/releases

For larger repos, provide a source token even with `--public` to get 5,000 req/hr:
```bash
vcopy large-org/big-repo my-org --public --issues --source-token ghp_xxx
```

The `--public` flag controls whether source auth is *required* — you can always optionally provide a `--source-token` alongside it for better rate limits on metadata operations.

## Verification Details

### Git Object Hash Verification
Every git object (commit, tree, blob) has a unique SHA hash. vcopy enumerates all objects on both source and target, verifying they match exactly.

### Branch/Tag Ref Comparison
All refs (branches and tags) are compared to ensure they point to the same commit SHAs on both source and target. Detects missing, extra, or diverged refs.

### Tree Hash Comparison
For each branch tip, the root tree hash is compared. This verifies the complete directory structure and file contents match.

### Commit Signature Verification
For signed commits (GPG or SSH), vcopy verifies that signatures are preserved and intact after copying. Reports warnings if signatures are lost.

### Bundle Checksum Verification
A git bundle is created from both source and target repos. SHA-256 checksums are computed and compared to verify byte-level transfer integrity.

## Flags Reference

| Flag | Default | Description |
|------|---------|-------------|
| `-h`, `--help` | | Show help and usage information |
| `--source-host` | `github.com` | Source GitHub host |
| `--target-host` | `github.com` | Target GitHub host |
| `--target-name` | same as source | Target repo name |
| `--auth-method` | `auto` | Auth method: `auto`, `gh`, `pat` |
| `--source-token` | | PAT for source |
| `--target-token` | | PAT for target |
| `--public` | `false` | Source repo is public (skip source auth) |
| `--issues` | `false` | Copy issues |
| `--pull-requests` | `false` | Copy pull requests |
| `--wiki` | `false` | Copy wiki |
| `--releases` | `false` | Copy releases and artifacts |
| `--all-metadata` | `false` | Copy all metadata |
| `--verify-only` | `false` | Skip copy, only verify |
| `--report` | | Path for JSON report |
| `--verbose` | `false` | Verbose output |
| `--dry-run` | `false` | Show plan without executing |

## Requirements

- **git** must be installed and available in PATH
- **gh** CLI (optional, for `auto` or `gh` auth methods)
- Network access to both source and target GitHub instances

## Security

- **Token input is hidden**: When entering PATs interactively, terminal echo is disabled so tokens are never visible on screen.
- **Git output is sanitized**: All git command output (stdout/stderr) is filtered to replace tokens with `[REDACTED]` before display, preventing credential leakage in verbose mode or error messages.
- **Tokens are never logged**: Tokens embedded in git clone URLs are stripped from any output shown to the user.
- **Temp files are cleaned up**: All temporary directories (bare clones, bundles, asset uploads) are removed after use via `defer`.
- **Private by default**: Target repositories are created as private.
