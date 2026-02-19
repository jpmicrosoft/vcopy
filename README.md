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
| `--source-host` | `github.com` | Source GitHub host |
| `--target-host` | `github.com` | Target GitHub host |
| `--target-name` | same as source | Target repo name |
| `--auth-method` | `auto` | Auth method: `auto`, `gh`, `pat` |
| `--source-token` | | PAT for source |
| `--target-token` | | PAT for target |
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
