# vcopy — Verified GitHub Repository Copy Tool

A CLI tool that copies GitHub repositories between organizations (Cloud and Enterprise) with comprehensive integrity verification.

## Quick Start

**You need:** `git` in PATH, a token with `repo` scope on both source and target, and optionally the [`gh` CLI](https://cli.github.com/) for automatic token detection. To build from source you also need Go 1.21+.

**Install:**

```bash
go install github.com/jpmicrosoft/vcopy/cmd/vcopy@latest
```

Or download a pre-built binary from [Releases](https://github.com/jpmicrosoft/vcopy/releases).

**Copy a repo:**

```bash
vcopy myorg/myrepo target-org
```

That's it. vcopy will authenticate, create the target as private, mirror all branches/tags/history, sync releases, run 5-layer integrity verification, and print a pass/fail report.

**Common variations:**

```bash
# Copy to GitHub Enterprise
vcopy myorg/myrepo target-org --target-host github.mycompany.com

# Copy a public repo (no source token needed)
vcopy golang/go my-org --public

# Code only — no tags, releases, or metadata
vcopy myorg/myrepo target-org --code-only

# Copy without source CI/CD workflows or Copilot config
vcopy myorg/myrepo target-org --no-workflows --no-copilot

# Provide tokens explicitly (for scripts/CI)
vcopy myorg/myrepo target-org --auth-method pat --source-token ghp_xxx --target-token ghp_yyy

# Dry run — see what would happen without doing anything
vcopy myorg/myrepo target-org --dry-run
```

**As a GitHub Action:**

```yaml
- uses: your-org/vcopy@v1
  with:
    source-repo: source-org/my-repo
    target-org: target-org
    source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
    target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

See [GitHub Action docs](action/README.md) for full setup guide and examples.

---

## Features

- **Verified mirroring** of all branches, tags, and commit history
- **Smart release sync**: tags and releases auto-sync; additive by default (preserves target-only content)
- **Code-only mode**: `--code-only` to copy branches/commits without tags or releases
- **Path exclusion**: `--no-workflows`, `--no-actions`, `--no-copilot`, `--no-github`, and `--exclude` to remove specific files/directories from the target
- **Batch copy**: `vcopy batch` to copy multiple repos from a source org using search filters, with prefix/suffix naming and resumable runs
- **5-layer integrity verification**:
  1. Git object hash verification (every commit, tree, blob)
  2. Branch/tag ref comparison
  3. Root tree hash comparison per branch
  4. GPG/SSH commit signature preservation check
  5. Git bundle integrity verification
- **Git LFS support**: `--lfs` flag to include LFS objects (auto-detects and warns if LFS is present)
- **Flexible authentication**: auto-detects `gh` CLI tokens, falls back to PAT
- **Metadata migration** (optional): issues, pull requests, wiki
- **Quick verification**: `--quick-verify` for fast ref + tree hash checks only
- **Incremental verification**: `--since` to verify only new objects since a date or SHA
- **Skip verification**: `--skip-verify` for copy-only workflows (verify later with `--verify-only`)
- **Attestation Signature**: `--sign` to GPG-sign the verification report for tamper-proof audit trails
- **Config file**: `--config` for repeatable YAML-based configuration
- **Retry with backoff**: automatic retry on transient network/API failures
- **Progress indicators**: animated spinners for long-running operations
- **Audit trail**: colored terminal report + optional JSON output
- **Cross-platform**: single binary for Windows, macOS, Linux

## Installation

The Quick Start above covers `go install`. For other methods:

```bash
# Build from source
git clone https://github.com/jpmicrosoft/vcopy.git
cd vcopy
go build -o vcopy ./cmd/vcopy

# Cross-platform builds (Linux, macOS, Windows × amd64/arm64)
make build-all   # outputs to bin/
```

> **Note:** Pre-built binaries from [Releases](https://github.com/jpmicrosoft/vcopy/releases) are named by platform (e.g., `vcopy-windows-amd64.exe`, `vcopy-linux-arm64`). You can run them directly or rename to `vcopy` (`vcopy.exe` on Windows) so the examples in this README work as-is.

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

Copies all branches, tags, commits, and releases:
```bash
vcopy myorg/myrepo target-org
```

### Code only (no tags or releases)

```bash
vcopy myorg/myrepo target-org --code-only
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
vcopy myorg/myrepo target-org --issues --wiki
```

### Copy without GitHub Actions workflows

When copying a repo to a different org, you may not want the source's CI/CD workflows running in the target. The `--no-workflows` flag removes the `.github/workflows/` directory after the copy:

```bash
vcopy myorg/myrepo target-org --no-workflows
```

This is especially useful when the source has workflows with secrets, environment references, or deployment steps that are specific to the source org and would fail or cause unintended side effects in the target.

### Copy without Copilot instructions and skills

Organizations often configure Copilot behavior with `.github/copilot-instructions.md` and `.github/copilot/` skill files. These are org-specific and usually shouldn't carry over:

```bash
vcopy myorg/myrepo target-org --no-copilot
```

### Combine workflow and Copilot exclusion

```bash
vcopy myorg/myrepo target-org --no-workflows --no-copilot
```

### Exclude custom paths

Use `--exclude` to remove any files or directories. Paths are comma-separated:

```bash
vcopy myorg/myrepo target-org --exclude vendor,docs/internal,scripts/deploy.sh
```

You can combine presets and custom exclusions:

```bash
vcopy myorg/myrepo target-org --no-workflows --no-copilot --exclude vendor,docs/internal
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
| Metadata copy (issues, PRs) | ⚠️ Works but rate-limited (60 req/hr) | Full rate limit (5,000 req/hr) |
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

## What Gets Verified

After copying a repository, vcopy runs **5 independent checks** to confirm nothing was lost or changed in transit. Think of it like a shipping manifest — every item is checked off:

| Check | What it answers | Analogy |
|-------|----------------|---------|
| **Ref Comparison** | Do all branches and tags exist and point to the same commits? | "Are all the shipping labels correct?" |
| **Object Hashes** | Does every commit, file, and folder have the exact same content? | "Is every item in the box identical to the original?" |
| **Tree Hashes** | Does each branch's directory structure and file contents match? | "Do the contents of each folder match?" |
| **Commit Signatures** | Are GPG/SSH signatures on commits still intact? | "Are all the wax seals unbroken?" |
| **Bundle Integrity** | Can both repos produce valid, equivalent git bundles? | "Can both warehouses produce identical inventories?" |

If **all 5 pass**, you have cryptographic proof the copy is identical to the source. If any fail, the report tells you exactly what differs.

> **Note**: In `--code-only` mode, Ref Comparison and Bundle Integrity are skipped because they depend on tags matching. The remaining 3 checks (Object Hashes, Tree Hashes, Commit Signatures) still run to verify branch integrity.

## Verification Technical Details

### 1. Branch/Tag Ref Comparison
Uses `git ls-remote` on both source and target to enumerate all `refs/heads/*` and `refs/tags/*`. Compares the SHA-1/SHA-256 commit hash each ref points to. Detects missing refs, extra refs, or refs pointing to different commits. GitHub's hidden `refs/pull/*` are excluded (see [Hidden Refs](#hidden-refs-refspull)).

### 2. Git Object Hash Verification
Clones both repos bare, then runs `git rev-list --objects --all` to enumerate every reachable object (commits, trees, blobs). Each object's SHA hash is its content-addressable identity — if even a single byte differs, the hash changes. All source object hashes are checked against the target to confirm a 1:1 match.

### 3. Tree Hash Comparison
For each branch head, computes `git rev-parse <branch>^{tree}` on both repos. The tree hash is a recursive hash of the entire directory structure at that branch tip — file names, permissions, and content hashes. If the tree hashes match, the working directories are byte-for-byte identical.

### 4. Commit Signature Verification
Uses `git log --all --format=%H %G?` on both repos to enumerate all commits and their signature status (Good, Bad, None, etc.). Compares the set of signed commit SHAs to ensure no signatures were dropped or corrupted during the mirror. Reports as a warning (not failure) if signatures differ, since some transfer methods may strip them.

### 5. Bundle Integrity Verification
Creates a `git bundle` from each repo (a self-contained archive of all refs and objects). Verifies each bundle passes `git bundle verify` (structural integrity). Then compares the refs listed inside each bundle using `git bundle list-heads` to confirm they match. SHA-256 checksums of each bundle file are included in the report for audit purposes, but are not compared directly because git bundle packing is non-deterministic.

## Flags Reference

| Flag | Default | Description |
|------|---------|-------------|
| `-h`, `--help` | | Show help and usage information |
| `-v`, `--version` | | Show vcopy version |
| `--config` | | Path to a YAML config file for repeatable setups (see [Config File](#config-file)) |
| `--source-host` | `github.com` | Hostname of the source GitHub instance (e.g., `github.mycompany.com` for Enterprise) |
| `--target-host` | `github.com` | Hostname of the target GitHub instance |
| `--target-name` | same as source | Name for the repo in the target org (defaults to the source repo's name) |
| `--auth-method` | `auto` | How to authenticate: `auto` tries gh CLI then PAT prompt, `gh` uses gh CLI only, `pat` uses tokens only |
| `--source-token` | | Personal Access Token for the source instance (avoids interactive prompt) |
| `--target-token` | | Personal Access Token for the target instance (avoids interactive prompt) |
| `--public` | `false` | Source repo is publicly accessible — skips source authentication entirely, only target credentials needed |
| `--lfs` | `false` | Include Git Large File Storage objects in the copy (requires `git-lfs` installed) |
| `--force` | `false` | Destructive mirror push to an existing target repo — overwrites all branches, tags, and releases. Requires `yes/no` confirmation unless `--non-interactive` is set |
| `--code-only` | `false` | Copy only branches and commits — skips tags, releases, and all metadata. Verification skips tag-dependent checks |
| `--issues` | `false` | Also migrate issues (titles, bodies, labels, comments) from source to target |
| `--pull-requests` | `false` | Also migrate pull requests as issues in the target (GitHub API does not support creating true PRs) |
| `--wiki` | `false` | Also copy the repository wiki (if one exists) |
| `--all-metadata` | `false` | Shorthand for `--issues --pull-requests --wiki` — copies all optional metadata |
| `--verify-only` | `false` | Run verification checks against an existing source and target without copying anything |
| `--skip-verify` | `false` | Copy the repo but skip all verification checks (you can verify later with `--verify-only`) |
| `--quick-verify` | `false` | Run only ref comparison and tree hash checks — faster but less thorough than full verification |
| `--since` | | Only verify objects created after this date (`2025-06-01`) or commit SHA — useful for incremental re-syncs |
| `--report` | | File path to write a detailed JSON verification report (e.g., `--report audit.json`) |
| `--sign` | | GPG key ID to sign the verification report for tamper-proof audit trails (requires `gpg` installed) |
| `--verbose` | `false` | Show detailed output for every step (git commands, API calls, skipped items) |
| `--dry-run` | `false` | Show what would happen without actually copying or modifying anything |
| `--non-interactive` | `false` | Skip confirmation prompts — required for CI/CD and automation (the GitHub Action sets this automatically) |
| `--no-workflows` | `false` | Exclude GitHub Actions workflows (`.github/workflows/`) from the target |
| `--no-actions` | `false` | Exclude GitHub Actions custom actions (`.github/actions/`) from the target |
| `--no-copilot` | `false` | Exclude Copilot instructions and skills (`.github/copilot-instructions.md`, `.github/copilot/`) from the target |
| `--no-github` | `false` | Exclude entire `.github/` directory from the target (supersedes `--no-workflows`, `--no-actions`, `--no-copilot`) |
| `--exclude` | | Comma-separated list of additional paths to exclude from the target (e.g., `--exclude vendor,docs/internal`). Can be repeated |

## Security

- **Token input is hidden**: When entering PATs interactively, terminal echo is disabled so tokens are never visible on screen.
- **Git output is sanitized**: All git command output (stdout/stderr) is filtered to replace tokens with `[REDACTED]` before display, preventing credential leakage in verbose mode or error messages.
- **Tokens are never logged**: Tokens embedded in git clone URLs are stripped from any output shown to the user.
- **Temp files are cleaned up**: All temporary directories (bare clones, bundles, asset uploads) are removed after use via `defer`.
- **Private by default**: Target repositories are created as private.
- **Input validation on `--since`**: The `--since` value is validated to prevent git flag injection; only hex SHAs and date strings are accepted.
- **Path traversal protection**: Repository names are sanitized (`filepath.Base` + `..` removal) before use in temp directory paths to prevent directory escape.
- **SSRF protection**: Release asset downloads validate URL scheme (https only), block private/internal network addresses (localhost, 10.x, 192.168.x, 169.254.x), and use a timeout-limited HTTP client.
- **Attestation uses proper GPG detached signatures**: Signing produces an armored detached signature; verification uses separate temp files for signature and data, matching GPG's expected `--verify <sig-file> <data-file>` protocol.
- **Nil-safe metadata migration**: Issue, PR, and comment formatting guards against nil user pointers (deleted/ghost GitHub accounts) to prevent panics during metadata copy.

## Existing Repository Safety (`--force`)

When the target repository already exists, vcopy uses **additive mode** by default:

| What happens | Default (additive) | With `--force` (destructive) |
|---|---|---|
| New branches from source | ✅ Added | ✅ Added |
| Existing branches in target | 🔄 Updated to match source | 🔄 Updated to match source |
| Branches only in target | ✅ Preserved | ❌ Deleted |
| New tags from source | ✅ Added | ✅ Added |
| Existing tags in target | ✅ Preserved | ❌ Overwritten |
| Existing releases in target | ✅ Preserved | ❌ Deleted and re-copied from source |
| New releases from source | ✅ Synced | ✅ Copied |

**Additive mode** (default when target exists):
```bash
# Safe: existing tags and releases in target are preserved
vcopy myorg/myrepo target-org
```

**Destructive mode** (`--force`): uses `git push --mirror` which replaces everything. Requires `yes/no` confirmation (skipped with `--non-interactive` or in the GitHub Action):
```bash
# WARNING: overwrites all branches, tags, and releases in target
vcopy myorg/myrepo target-org --force
```

> ⚠️ **WARNING**: `--force` will **permanently delete** any branches, tags, or commits in the target that do not exist in the source. This cannot be undone.

## Path Exclusion

You can exclude specific files and directories from the target repository after copying. This is useful when the source contains GitHub Actions workflows, Copilot instructions, or other org-specific configuration that shouldn't carry over to the target.

### Why exclude paths?

| Scenario | How it's handled |
|----------|-----------------|
| Source has CI/CD workflows that reference secrets, environments, or deploy targets specific to the source org | Use `--no-workflows` — removes `.github/workflows/` |
| Source has custom composite actions specific to the source org | Use `--no-actions` — removes `.github/actions/` |
| Source has Copilot instructions or custom skills that are org-specific | Use `--no-copilot` — removes `.github/copilot-instructions.md` and `.github/copilot/` |
| You want to remove the entire `.github/` directory (issue templates, PR templates, workflows, actions, copilot, etc.) | Use `--no-github` — removes `.github/` (supersedes `--no-workflows`, `--no-actions`, `--no-copilot`) |
| Source has vendored dependencies, internal docs, or other paths you don't want in the target | Use `--exclude` — removes any paths you specify |
| Source has CODEOWNERS that reference teams/users from the source org | **Automatic** — `CODEOWNERS`, `.github/CODEOWNERS`, and `docs/CODEOWNERS` are always removed because they reference teams and users that won't exist in the target org. This prevents broken branch protection rules and review assignment failures |

### How it works (step by step)

Path exclusion is a **post-verification operation**. Here's exactly what happens:

1. **vcopy mirrors the full repository** — all branches, tags, commits, and history are pushed to the target (this is the standard copy step, identical to running without exclusion flags)
2. **Verification runs** — the 5-layer integrity check compares source and target to confirm the mirror is exact. This happens **before** any paths are removed, so verification compares the 1:1 mirror
3. **vcopy shallow-clones the target** — a `--depth 1` clone of the target's default branch is made to a temp directory
4. **Excluded paths are deleted** — `git rm -rf` removes the specified files/directories from the working tree. CODEOWNERS files are always included in this step.
5. **A cleanup commit is pushed** — a single commit with the message `vcopy: remove excluded paths` is pushed to the target's default branch. The commit message lists every path that was removed
6. **Temp directory is cleaned up** — all temporary files are deleted

```
Source Repo            Mirror + Verify              Target Repo (final)
┌────────────────┐     ┌────────────────┐           ┌────────────────┐
│ .github/       │────▶│ .github/       │ ✅ PASS   │ (removed)      │
│   workflows/   │     │   workflows/   │──verify──▶│                │
│   actions/     │     │   actions/     │           │                │
│   copilot/     │     │   copilot/     │           │                │
│   CODEOWNERS   │     │   CODEOWNERS   │           │                │
│ src/           │────▶│ src/           │           │ src/           │
│ README.md      │────▶│ README.md      │           │ README.md      │
│ All history    │────▶│ All history    │           │ All history +  │
│                │     │                │           │ cleanup commit │
└────────────────┘     └────────────────┘           └────────────────┘
                        Step 1: mirror              Step 2: exclude
                        Step 2: verify ✅            (after verify)
```

### What this means for you

- **Full git history is preserved** — every original commit, branch, and tag from the source exists in the target. The cleanup only affects the latest working tree on the default branch
- **Verification passes** — the 5-layer check runs against the exact mirror before any paths are removed, so it compares source-to-target 1:1
- **The cleanup is transparent** — anyone looking at the target repo can see exactly what was removed by inspecting the `vcopy: remove excluded paths` commit
- **Excluded paths exist in history** — the files are removed from the current tree, but they still exist in older commits. If you need them scrubbed from history entirely, you'd need a separate tool like `git filter-repo`
- **Non-existent paths are skipped** — if a path doesn't exist in the source (e.g., the repo has no `.github/workflows/`), the flag is silently ignored
- **CODEOWNERS is always removed** — on every copy, `CODEOWNERS` files are removed from all standard locations (`CODEOWNERS`, `.github/CODEOWNERS`, `docs/CODEOWNERS`) because they reference source org teams and users that won't exist in the target

### Preset flags

| Flag | Paths removed | Use case |
|------|--------------|----------|
| `--no-workflows` | `.github/workflows/` | Source CI/CD workflows shouldn't run in target org |
| `--no-actions` | `.github/actions/` | Custom composite actions are org-specific |
| `--no-copilot` | `.github/copilot-instructions.md`, `.github/copilot/` | Copilot config is org-specific |
| `--no-github` | `.github/` (entire directory) | Remove all GitHub config (supersedes the three flags above) |
| *(always)* | `CODEOWNERS`, `.github/CODEOWNERS`, `docs/CODEOWNERS` | Source team/user references would break branch protection in target |

### Custom paths with `--exclude`

Use `--exclude` to remove any files or directories by their repo-relative path:

```bash
# Remove a single directory
vcopy myorg/myrepo target-org --exclude vendor

# Remove multiple paths (comma-separated)
vcopy myorg/myrepo target-org --exclude vendor,docs/internal,scripts/deploy.sh

# Combine with presets
vcopy myorg/myrepo target-org --no-workflows --no-copilot --exclude vendor
```

**Path validation rules:**
- Paths must be **relative** to the repository root (no leading `/`)
- Paths must not contain `..` (directory traversal is blocked)
- Paths must not start with `-` (flag injection is blocked)
- Backslashes are normalised to forward slashes automatically

### Config file

All exclusion flags are available in the YAML config:

```yaml
exclude:
  workflows: true   # same as --no-workflows
  copilot: true     # same as --no-copilot
  paths:            # same as --exclude
    - vendor
    - docs/internal
    - scripts/deploy.sh
```

## Batch Copy

Copy multiple repositories from a source org to a target org in a single command. Repos are discovered automatically via GitHub API search.

### Usage

```bash
vcopy batch <source-org> <target-org> --search "<name-filter>" [flags]
```

### Azure Terraform AVM Example

```bash
# Preview all Azure Terraform AVM modules (dry-run)
vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-" --public --no-github --dry-run

# Copy all AVM modules (resource + pattern)
vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-" --public --no-github --skip-verify

# Copy only AVM resource modules
vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-res-" --public --no-github

# Copy with a prefix on target names
vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-" --prefix "avm-" --public --no-github

# Resume an interrupted batch
vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-" --skip-existing --public --no-github
```

### Generic Examples

```bash
# Copy all repos matching "service-" from any org
vcopy batch mycompany backup-org --search "service-" --skip-verify

# Cross-host batch (Enterprise to Cloud)
vcopy batch corp-org cloud-org --search "platform-" --source-host ghes.corp.com --target-host github.com

# Add suffix to all target names
vcopy batch source-org target-org --search "api-" --suffix "-imported"
```

### Batch Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--search` | *(required)* | Repository name filter (matched against repo names in source org) |
| `--prefix` | | Prefix to prepend to each target repo name |
| `--suffix` | | Suffix to append to each target repo name |
| `--skip-existing` | `false` | Skip repos that already exist in the target org (useful for resuming interrupted batches) |
| `--dry-run` | `false` | Preview the full source→target mapping without copying anything |

All standard vcopy flags (`--public`, `--no-github`, `--skip-verify`, `--code-only`, etc.) are also available and apply to every repo in the batch.

### Target Naming

Target repo names follow the pattern `{prefix}{source-name}{suffix}`:

| Source Name | Prefix | Suffix | Target Name |
|-------------|--------|--------|-------------|
| `terraform-azurerm-avm-res-cache-redis` | | | `terraform-azurerm-avm-res-cache-redis` |
| `terraform-azurerm-avm-res-cache-redis` | `avm-` | | `avm-terraform-azurerm-avm-res-cache-redis` |
| `terraform-azurerm-avm-res-cache-redis` | | `-internal` | `terraform-azurerm-avm-res-cache-redis-internal` |
| `my-service` | `imported-` | `-v2` | `imported-my-service-v2` |

### Behavior

- **Sequential execution**: Repos are copied one at a time (rate-limit friendly)
- **Error handling**: If one repo fails, the batch skips it and continues to the next
- **Progress**: Prints `[N/total] Copying repo-name...` for each repo
- **Summary**: At the end, prints a report of succeeded/failed/skipped counts
- **Resumable**: Use `--skip-existing` to skip repos already created (e.g., after a partial run)

## Hidden Refs (refs/pull/*)

GitHub creates read-only `refs/pull/*/head` and `refs/pull/*/merge` refs for every pull request. These are internal to GitHub and cannot be pushed to another repository.

vcopy automatically handles this:
- **During copy**: PR refs are stripped from the bare clone before mirror push (main repo and wiki)
- **During verification**: PR refs are excluded from ref comparison, object/tree/signature verification clones, and bundle integrity checks

This means all branches, tags, and commit history are copied and verified, but PR refs are intentionally excluded. If you need PR metadata, use the `--pull-requests` flag to migrate PRs as issues.

## Git LFS Support

Use `--lfs` to include Git LFS objects in the copy:

```bash
vcopy myorg/lfs-repo target-org --lfs
```

If a repository uses LFS but `--lfs` is not specified, vcopy will detect this and print a warning. Without `--lfs`, LFS pointer files are copied but the actual large files are not.

**Requirements**: `git-lfs` must be installed and available in PATH.

## Quick and Incremental Verification

### Quick verify (fast)

Runs only ref comparison and tree hash checks, skipping the slower object enumeration, signature verification, and bundle integrity:

```bash
vcopy myorg/myrepo target-org --quick-verify
```

### Incremental verify

Only verifies objects created after a given date or commit SHA. Useful for repeat syncs:

```bash
vcopy myorg/myrepo target-org --verify-only --since "2025-06-01"
vcopy myorg/myrepo target-org --verify-only --since abc123def
```

### Skip verification

Copy now, verify later:

```bash
vcopy myorg/myrepo target-org --skip-verify
# Later:
vcopy myorg/myrepo target-org --verify-only
```

## Attestation Signature

Sign the verification report with GPG to create a tamper-proof audit trail:

```bash
vcopy myorg/myrepo target-org --report audit.json --sign "your-gpg-key-id"
```

The signed report includes an `attestation` field containing the GPG key ID and detached signature over the SHA-256 hash of the report data. This proves:
- Who verified the copy (the key owner)
- When the verification was performed
- That the report has not been modified since signing

**Requirements**: `gpg` must be installed with the specified key available.

## Config File

For repeated use or scheduled syncs, use a YAML config file instead of flags:

```bash
vcopy --config mirror.yaml
```

CLI flags override config file values. Example `mirror.yaml`:

```yaml
source:
  repo: myorg/myrepo
  host: github.com
  public: false

target:
  org: target-org
  host: github.mycompany.com
  name: my-mirror

auth:
  method: auto

copy:
  issues: true
  wiki: true

verify:
  quick: true

lfs: true
force: false     # Destructive mirror push (prompts unless non_interactive is true)
code_only: false # Copy only branches/commits (no tags or releases)

exclude:
  workflows: true  # Exclude .github/workflows/ from target
  copilot: true    # Exclude Copilot instructions/skills from target
  paths:           # Additional paths to exclude
    - vendor

report:
  path: audit.json
  sign_key: ABCD1234
```

See `example-config.yaml` in the repository for a complete template.

## Retry Behavior

vcopy automatically retries failed git operations and API calls with exponential backoff:
- **Max attempts**: 3
- **Initial wait**: 1 second
- **Max wait**: 30 seconds

This handles transient network failures and GitHub API rate limits gracefully.

## GitHub Action

vcopy is also available as a reusable GitHub Action. Clone this repo into your organization, create a release, and use it directly in your workflows.

### Quick Start

```yaml
- uses: your-org/vcopy@v1
  with:
    source-repo: source-org/my-repo
    target-org: target-org
    source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
    target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Setup

1. **Clone this repo** into your organization
2. **Create a release** — push a version tag (`git tag v1.0.0 && git push origin v1.0.0`) to trigger the release workflow, which builds cross-platform binaries and publishes them
3. **Reference the action** from any workflow in your org: `uses: your-org/vcopy@v1`

All CLI flags are available as action inputs (`force`, `code-only`, `lfs`, `all-metadata`, etc.). The action runs in `--non-interactive` mode automatically.

For full documentation, inputs/outputs reference, and example workflows, see **[action/README.md](action/README.md)**.

## Troubleshooting / FAQ

**The release binary is named `vcopy-windows-amd64.exe`, not `vcopy.exe`.**
Pre-built binaries include the platform and architecture in the filename. You can either run it directly (`./vcopy-windows-amd64.exe myorg/repo target-org`) or rename it to `vcopy.exe` (or `vcopy` on Linux/macOS) so the examples in this README work as-is. No installation step is required — it's a standalone executable.

**Authentication fails with "401 Unauthorized".**
Ensure your token has the `repo` scope. For public source repos, use `--public` to skip source authentication entirely.

**"Must have admin rights to Repository" when using `--force`.**
Your target token needs admin-level access to the target repo. For organization-owned repos, ensure the token has `repo` scope and the user is an org owner or has admin role on the repo.

**Verification fails on `refs/pull/*` refs.**
GitHub creates hidden `refs/pull/*` refs for pull requests. These are read-only and cannot be pushed. vcopy automatically excludes them during verification — if you see failures, ensure you're on the latest version.
