# vcopy — Verified GitHub Repository Copy

[![CI](https://github.com/jpmicrosoft/vcopy/actions/workflows/ci.yml/badge.svg)](https://github.com/jpmicrosoft/vcopy/actions/workflows/ci.yml)
[![Release](https://github.com/jpmicrosoft/vcopy/actions/workflows/release.yml/badge.svg)](https://github.com/jpmicrosoft/vcopy/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/jpmicrosoft/vcopy)](https://goreportcard.com/report/github.com/jpmicrosoft/vcopy)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Copy GitHub repositories between organizations — Cloud, Enterprise, or both — with 5-layer cryptographic integrity verification.**

vcopy mirrors branches, tags, commits, releases, and optionally issues/PRs/wiki, then runs five independent checks to prove the copy is bit-for-bit identical to the source. Available as a **CLI tool** and a **GitHub Action**.

---

## Quick Start

**Prerequisites:** `git` in PATH and a token with `repo` scope on both source and target. Optionally, the [`gh` CLI](https://cli.github.com/) for automatic token detection. Go 1.21+ to build from source.

### Install

```bash
go install github.com/jpmicrosoft/vcopy/cmd/vcopy@latest
```

Or download a pre-built binary from [Releases](https://github.com/jpmicrosoft/vcopy/releases).

### Copy a repo

```bash
vcopy myorg/myrepo target-org
```

That's it. vcopy authenticates, creates the target as private, mirrors all branches/tags/history, syncs releases, runs 5-layer integrity verification, and prints a pass/fail report.

### Common variations

```bash
# Copy to GitHub Enterprise
vcopy myorg/myrepo target-org --target-host github.mycompany.com

# Copy a public repo (no source token needed)
vcopy golang/go my-org --public-source

# Code only — no tags, releases, or metadata
vcopy myorg/myrepo target-org --code-only

# Strip CI/CD and Copilot config from the target
vcopy myorg/myrepo target-org --no-workflows --no-copilot

# Explicit tokens for scripts and CI
vcopy myorg/myrepo target-org --auth-method pat --source-token ghp_xxx --target-token ghp_yyy

# Dry run — preview without making changes
vcopy myorg/myrepo target-org --dry-run
```

### As a GitHub Action

```yaml
- uses: jpmicrosoft/vcopy@v1
  with:
    source-repo: source-org/my-repo
    target-org: target-org
    source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
    target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

See the [Action documentation](action/README.md) for full setup and examples.

---

## Features

| Category | Capabilities |
|----------|-------------|
| **Mirroring** | All branches, tags, commit history, and releases. Additive by default (preserves target-only content). |
| **Integrity** | 5-layer verification: object hashes, ref comparison, tree hashes, commit signatures, bundle integrity. |
| **Batch copy** | `vcopy batch` discovers and copies multiple repos by search filter with prefix/suffix naming and resumable runs. |
| **Path control** | `--no-workflows`, `--no-actions`, `--no-copilot`, `--no-github`, `--exclude` — strip org-specific files from the target. |
| **Metadata** | Optionally migrate issues, pull requests (as issues), and wiki. |
| **Verification modes** | `--quick-verify` (fast), `--since` (incremental), `--skip-verify` / `--verify-only` (deferred). |
| **Attestation** | `--sign` GPG-signs the JSON verification report for tamper-proof audit trails. |
| **Reporting** | JSON verification reports. In batch mode: combined report + optional per-repo reports. |
| **LFS** | `--lfs` includes Git LFS objects (auto-detects and warns). |
| **Auth** | Auto-detects `gh` CLI tokens, falls back to PAT. `--public-source` for open-source repos. |
| **Resilience** | Automatic retry with exponential backoff on network/API failures. Auto-retry on GitHub rate limits — 403s (primary) and 429s (secondary/abuse) — sleeps until reset. |
| **Config** | YAML config file (`--config`) for repeatable setups. |
| **Cross-platform** | Single binary for Windows, macOS, Linux (amd64 + arm64). |
| **Action** | Reusable GitHub Action with all CLI capabilities. |

## Installation

### Pre-built binaries (recommended)

Download the latest binary for your platform from [Releases](https://github.com/jpmicrosoft/vcopy/releases). Binaries are named by platform (e.g., `vcopy-windows-amd64.exe`, `vcopy-linux-arm64`). Rename to `vcopy` (`vcopy.exe` on Windows) for convenience, or run directly.

### Build from source

```bash
git clone https://github.com/jpmicrosoft/vcopy.git
cd vcopy
go build -o vcopy ./cmd/vcopy
```

### Cross-compile all platforms

```bash
make build-all   # outputs to bin/
```

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

Use the `--public-source` flag when the source repository is publicly accessible. This skips source authentication entirely — you only need credentials for the target.

### How `--public-source` works

| Feature | With `--public-source` | Without `--public-source` |
|---------|----------------|-------------------|
| Git clone/push | ✅ No source token needed | Requires source token |
| Verification (refs, objects, trees, bundle) | ✅ No source token needed | Requires source token |
| Commit signature verification | ✅ No source token needed | Requires source token |
| Metadata copy (issues, PRs) | ⚠️ Works but rate-limited (60 req/hr) | Full rate limit (5,000 req/hr) |
| Wiki copy | ✅ No source token needed | Requires source token |

### Examples

Copy a public repo (no source auth needed):
```bash
vcopy golang/go my-org --public-source
```

Copy a public repo with metadata (rate-limited for large repos):
```bash
vcopy golang/go my-org --public-source --all-metadata
```

Copy a public repo to an Enterprise instance:
```bash
vcopy kubernetes/kubernetes my-org --public-source --target-host github.mycompany.com
```

Copy a public repo as an internal repo in the target org:

```bash
vcopy kubernetes/kubernetes my-org --public-source --visibility internal
```

### Rate limiting with public repos

When copying metadata from public repos without a source token, the GitHub API allows only **60 requests per hour** (unauthenticated). This is usually fine for:
- Repos with fewer than ~50 issues/PRs/releases

For larger repos, provide a source token even with `--public-source` to get 5,000 req/hr:
```bash
vcopy large-org/big-repo my-org --public-source --issues --source-token ghp_xxx
```

The `--public-source` flag controls whether source auth is *required* — you can always optionally provide a `--source-token` alongside it for better rate limits on metadata operations.

> **Note:** All API calls (authenticated or not) automatically retry on rate limit responses (403 primary and 429 secondary/abuse) by sleeping until the reset time. With unauthenticated access, this means metadata operations may pause frequently but will complete rather than fail. See [Retry Behavior](#retry-behavior) for details.

## Integrity Verification

After copying, vcopy runs **5 independent checks** to cryptographically prove nothing was lost or altered in transit:

| # | Check | What it proves |
|---|-------|---------------|
| 1 | **Ref Comparison** | Every branch and tag points to the same commit SHA |
| 2 | **Object Hashes** | Every commit, file, and directory has identical content (SHA-based) |
| 3 | **Tree Hashes** | Each branch's directory structure and file contents match byte-for-byte |
| 4 | **Commit Signatures** | GPG/SSH signatures on commits survived the transfer |
| 5 | **Bundle Integrity** | Both repos produce structurally valid, equivalent git bundles |

**All 5 pass → cryptographic proof the copy is identical.** If any check reports WARN, all source content was verified but the target contains additional content or excluded refs. If any check reports FAIL, the report tells you exactly what differs.

> In `--code-only` mode, checks 1 and 5 are skipped (they depend on tags). The remaining 3 checks still verify branch integrity.

> **WARN status — extra target content (additive mode):** In additive mode (the default), the target may contain branches, tags, objects, or refs from prior copy runs or manual commits that don't exist in the source. These extras are reported as **WARN**, not FAIL, because they don't indicate data loss — all source content is still present. Only *missing* source content in the target causes FAIL.

> **WARN status — rejected branches:** When the target remote rejects specific branches during push (e.g., due to org rulesets or branch protection rules), vcopy automatically excludes those branches from verification and reports **WARN** instead of FAIL. This means all successfully pushed content was verified, but some source branches could not be pushed. Rejected branches are listed in the push output.

## Verification Technical Details

### 1. Branch/Tag Ref Comparison
Uses `git ls-remote` on both source and target to enumerate all `refs/heads/*` and `refs/tags/*`. Compares the SHA-1/SHA-256 commit hash each ref points to. Detects missing refs, extra refs, or refs pointing to different commits. GitHub's hidden `refs/pull/*` are excluded (see [Hidden Refs](#hidden-refs-refspull)). Branches rejected by the remote during push (due to branch protection or org rulesets) are automatically excluded from comparison and cause a WARN result.

### 2. Git Object Hash Verification
Clones both repos bare, then runs `git rev-list --objects --all` to enumerate every reachable object (commits, trees, blobs). Each object's SHA hash is its content-addressable identity — if even a single byte differs, the hash changes. All source object hashes must be present in the target (missing source objects = FAIL). Extra objects in the target (from prior runs or cleanup commits in additive mode) produce a WARN, not a failure.

### 3. Tree Hash Comparison
For each branch head, computes `git rev-parse <branch>^{tree}` on both repos. The tree hash is a recursive hash of the entire directory structure at that branch tip — file names, permissions, and content hashes. If the tree hashes match, the working directories are byte-for-byte identical.

### 4. Commit Signature Verification
Uses `git log --all --format=%H %G?` on both repos to enumerate all commits and their signature status (Good, Bad, None, etc.). Compares the set of signed commit SHAs to ensure no signatures were dropped or corrupted during the mirror. Reports as a warning (not failure) if signatures differ, since some transfer methods may strip them.

### 5. Bundle Integrity Verification
Creates a `git bundle` from each repo (a self-contained archive of all refs and objects). Verifies each bundle passes `git bundle verify` (structural integrity). Then compares the refs listed inside each bundle using `git bundle list-heads` — all source bundle refs must be present with matching SHAs in the target bundle (missing or mismatched = FAIL). Extra refs in the target bundle (from prior runs in additive mode) produce a WARN. SHA-256 checksums of each bundle file are included in the report for audit purposes, but are not compared directly because git bundle packing is non-deterministic.

## Verification Failure Troubleshooting

When a verification check reports **FAIL** or **WARN**, the tables below list the most common causes and whether each represents a real integrity problem or a benign condition you can safely accept.

> **Why timing matters:** Each verification check independently queries or clones the source and target repositories. The 5 checks run sequentially, so there is a window (typically seconds to minutes depending on repo size) during which the source may receive new commits. When this happens, earlier checks may see one state of the source while later checks see a different state. This is the most common cause of isolated failures — especially for the bundle check, which runs last.

> **Tip:** Re-run the failing repo with `--verbose` to see exactly which refs, objects, or SHAs differ.

### 1. Branch/Tag Ref Comparison — Failure Causes

| Cause | Real problem? | What to do |
|-------|:---:|------------|
| **Target branch protection rejected a branch during push** | No | vcopy auto-excludes rejected branches (WARN). If you see FAIL instead, check that the rejected branch is listed in the push output — it should be excluded automatically. Re-run on latest version if not. |
| **Source repo was updated after the copy** | No | The source received new commits between copy and verification. Re-run with `--verify-only` to compare current state, or accept if you know the source moved. |
| **Incomplete push (network interruption)** | Yes | Some branches or tags didn't reach the target. Re-run the copy (additive mode won't destroy existing content). |
| **Token lacks push permissions** | Yes | Target token needs `repo` scope with write access. Check for 403/permission errors in the output. |

### 2. Git Object Hash Verification — Failure Causes

| Cause | Real problem? | What to do |
|-------|:---:|------------|
| **Source repo received new commits during verification** | No | Each check clones independently. If the source was updated between the ref check and the object check, objects may differ. Re-run with `--verify-only`. |
| **Incomplete push** | Yes | Some objects weren't transferred. Re-run the copy. |
| **Target garbage collection removed unreachable objects** (self-hosted targets) | Depends | If a self-hosted GitHub Enterprise target ran `git gc` aggressively, unreachable objects may have been pruned. GitHub.com manages GC automatically. Re-run the copy to restore them. |
| **Branch protection prevented pushing all content** | No | Objects only reachable from rejected branches are excluded. If the check still fails, a non-standard ref namespace may contain source-only objects (see Bundle section below). |

### 3. Tree Hash Comparison — Failure Causes

| Cause | Real problem? | What to do |
|-------|:---:|------------|
| **Source branch received new commits during verification** | No | The tree hash at the branch tip changed. Re-run with `--verify-only`. |
| **Branch was force-pushed on source after copy** | No | Source history was rewritten — the tree at the branch tip differs. Re-copy if you need the latest state. |
| **Incomplete push** | Yes | The branch tip commit or its tree wasn't fully transferred. Re-run the copy. |
| **CODEOWNERS or path exclusion applied** | No | Post-copy cleanup commits change the target's tree. These run *after* verification, so they shouldn't cause failures. If they do, it indicates a bug — please report it. |

### 4. Commit Signature Verification — Failure and Warning Causes

This check produces **WARN** (not FAIL) when signatures are lost — this is expected because additive push preserves signatures in most cases. **FAIL** only occurs for infrastructure errors.

| Cause | Status | Real problem? | What to do |
|-------|:------:|:---:|------------|
| **Clone or git command failure** | FAIL | Yes | Infrastructure error (network, auth, corrupted repo). Re-run the copy. |
| **Signatures not preserved during transfer** | WARN | Depends | Some transfer methods can strip GPG/SSH commit signatures. If you need signatures preserved, verify with `git log --show-signature` on the target. |
| **GPG/SSH keys not available in verification environment** | — | No | Not a problem. The check compares signature *presence* (`%G?` status ≠ `N`), not cryptographic validity — missing keys return `E` (cannot check), which still counts as "signed." |
| **Source has signed commits, target has same commits unsigned** | WARN | Depends | Can happen if commits were recreated (rebase, amend). Additive push should preserve signatures — if missing, it may indicate the push method altered commits. |

### 5. Bundle Integrity Verification — Failure Causes

| Cause | Real problem? | What to do |
|-------|:---:|------------|
| **Source repo updated during verification (most common)** | No | The bundle check runs last. If the source received a push between earlier checks and the bundle check, the bundle sees newer source content the target doesn't have. All other checks pass but the bundle fails. **Re-run with `--verify-only` to confirm.** |
| **Non-standard refs in source** (`refs/notes/*`, `refs/replace/*`) | Unlikely | Bundle verification uses `git clone --bare`, which only fetches `refs/heads/*` and `refs/tags/*` — non-standard namespaces are not included. This scenario is theoretically possible if the remote advertises them, but uncommon in practice. |
| **Branch protection rejected a branch** | No | Rejected branches are excluded from the source bundle. If the check still fails, see "source updated during verification" above. |
| **Corrupted packfile during transfer** | Yes | Rare. The bundle verification (`git bundle verify`) catches structural corruption. If the bundle itself is invalid, the error message will say "bundle verify failed." Re-run the copy. |
| **Bundle creation fails on empty repo** | — | If the target has no refs at all (all branches rejected, no tags), bundle creation may fail. This is reported as "Target bundle failed" — not an integrity issue. |

### Quick Reference: Is My Failure Safe to Accept?

| Scenario | Safe? | Explanation |
|----------|:-----:|-------------|
| All checks PASS except Bundle → FAIL | Usually yes | Most commonly caused by the source repo receiving a push during verification. Re-run `--verify-only` to confirm. |
| Ref Comparison → FAIL, others PASS | Investigate | A ref was missing or mismatched. Could be source updated or incomplete push. |
| Object Hashes → FAIL, others PASS | Investigate | Objects are missing. May need to re-copy. |
| Tree Comparison → FAIL | Investigate | Branch content differs. Check if source was force-pushed. |
| Signatures → WARN, others PASS | Yes | Signature preservation is best-effort. Content integrity is confirmed by the other checks. Lost signatures produce WARN, not FAIL. |
| Signatures → FAIL, others PASS | Investigate | FAIL in signatures means a clone or git command failed — infrastructure error, not a signature issue. |
| Multiple checks → FAIL | Investigate | Likely an incomplete copy or network issue. Re-run the copy. |

### Mode-Dependent Skip Behavior

| Check | Full mode | Quick mode (`--quick-verify`) | Code-only (`--code-only`) |
|-------|:---------:|:---:|:---:|
| Branch/Tag Ref Comparison | ✓ Runs | ✓ Runs | ⊘ Skipped (tags not copied) |
| Git Object Hash Verification | ✓ Runs | ⊘ Skipped | ✓ Runs |
| Tree Hash Comparison | ✓ Runs | ✓ Runs | ✓ Runs |
| Commit Signature Verification | ✓ Runs | ⊘ Skipped | ✓ Runs |
| Bundle Integrity Verification | ✓ Runs | ⊘ Skipped | ⊘ Skipped (tags not copied) |

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
| `--public-source` | `false` | Source repo is publicly accessible — skips source authentication entirely, only target credentials needed |
| `--visibility` | `private` | Target repo visibility: `private`, `public`, or `internal` |
| `--lfs` | `false` | Include Git Large File Storage objects in the copy (requires `git-lfs` installed) |
| `--force` | `false` | Destructive mirror push to an existing target repo — overwrites all branches, tags, and releases. Requires `yes/no` confirmation unless `--non-interactive` is set |
| `--code-only` | `false` | Copy only branches and commits — skips tags, releases, and all metadata. Verification skips tag-dependent checks |
| `--issues` | `false` | Also migrate issues (titles, bodies, labels, comments) from source to target |
| `--pull-requests` | `false` | Also migrate pull requests as issues in the target (GitHub API does not support creating true PRs) |
| `--wiki` | `false` | Also copy the repository wiki (if one exists) |
| `--all-metadata` | `false` | Shorthand for `--issues --pull-requests --wiki` — copies all optional metadata |
| `--verify-only` | `false` | Run verification checks against an existing source and target without copying anything. Cannot be combined with `--skip-verify` |
| `--skip-verify` | `false` | Copy the repo but skip all verification checks. Verify later in a separate run with `--verify-only`. Cannot be combined with `--verify-only` |
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

> **Verification in additive mode:** Because the target may contain extra content from prior runs (preserved branches, tags, objects), verification tolerates these extras and reports **WARN** rather than FAIL. Only missing source content in the target causes a failure. See [Integrity Verification](#integrity-verification).

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
vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-" --public-source --no-github --dry-run

# Copy all AVM modules (resource + pattern)
vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-" --public-source --no-github --skip-verify

# Copy only AVM resource modules
vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-res-" --public-source --no-github

# Copy with a prefix on target names
vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-" --prefix "avm-" --public-source --no-github

# Resume an interrupted batch
vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-" --skip-existing --public-source --no-github
```

### Generic Examples

```bash
# Copy all repos matching "service-" from any org
vcopy batch mycompany backup-org --search "service-" --skip-verify

# Cross-host batch (Enterprise to Cloud)
vcopy batch corp-org cloud-org --search "platform-" --source-host ghes.corp.com --target-host github.com

# Add suffix to all target names
vcopy batch source-org target-org --search "api-" --suffix "-imported"

# Batch copy with combined report
vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-" --public-source --no-github --report batch-audit.json

# Batch copy with combined + per-repo reports
vcopy batch Azure jpmicrosoft --search "terraform-azurerm-avm-" --public-source --report batch-audit.json --per-repo-report
```

### Batch Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--search` | *(required)* | Repository name filter (matched against repo names in source org) |
| `--prefix` | | Prefix to prepend to each target repo name |
| `--suffix` | | Suffix to append to each target repo name |
| `--skip-existing` | `false` | Skip repos that already exist in the target org (useful for resuming interrupted batches) |
| `--sync` | `false` | Update existing repos (additive push + incremental release sync) instead of skipping them |
| `--dry-run` | `false` | Preview the full source→target mapping without copying anything |
| `--report` | | Path to write a combined JSON batch report with all repo results |
| `--per-repo-report` | `false` | Also write individual JSON reports per repo (e.g., `report-reponame.json`) |
| `--batch-delay` | `3s` | Delay between repos to avoid GitHub's secondary rate limits (e.g., `5s`, `0s` to disable) |

All standard vcopy flags (`--public-source`, `--visibility`, `--no-github`, `--skip-verify`, `--code-only`, etc.) are also available and apply to every repo in the batch.

### Target Naming

Target repo names follow the pattern `{prefix}{source-name}{suffix}`:

| Source Name | Prefix | Suffix | Target Name |
|-------------|--------|--------|-------------|
| `terraform-azurerm-avm-res-cache-redis` | | | `terraform-azurerm-avm-res-cache-redis` |
| `terraform-azurerm-avm-res-cache-redis` | `avm-` | | `avm-terraform-azurerm-avm-res-cache-redis` |
| `terraform-azurerm-avm-res-cache-redis` | | `-internal` | `terraform-azurerm-avm-res-cache-redis-internal` |
| `my-service` | `imported-` | `-v2` | `imported-my-service-v2` |

### Behavior

- **Sequential execution**: Repos are copied one at a time with a configurable delay (`--batch-delay`, default 3s) to avoid secondary rate limits
- **Secondary rate limit recovery**: If GitHub's secondary rate limit is hit, vcopy waits for the reset period and retries automatically
- **Error handling**: If one repo fails, the batch skips it and continues to the next
- **Progress**: Prints `[N/total] Copying repo-name...` for each repo
- **Summary**: At the end, prints a report of succeeded/failed/skipped counts (plus releases-skipped if any were skipped due to rate limits)
- **Resumable**: Use `--skip-existing` to skip repos already created (e.g., after a partial run)
- **Sync mode**: Use `--sync` to update existing repos — branches are force-updated to match the source, while new tags and releases are added incrementally (existing tags and releases are preserved). New repos are created normally. Mutually exclusive with `--skip-existing`.
- **Reporting**: Use `--report` to write a combined JSON report; add `--per-repo-report` for individual files per repo

#### Batch Report JSON Schema

The combined batch report (`--report`) has this structure:

```json
{
  "source_org": "Azure",
  "target_org": "my-backup-org",
  "source_host": "github.com",
  "target_host": "github.com",
  "search_filter": "terraform-azurerm-avm-",
  "timestamp": "2025-07-15T12:00:00Z",
  "summary": {
    "total": 10,
    "succeeded": 8,
    "failed": 1,
    "skipped": 1,
    "releases_skipped": 2
  },
  "repos": [
    {
      "source_repo": "Azure/terraform-azurerm-avm-res-cache-redis",
      "target_repo": "my-backup-org/terraform-azurerm-avm-res-cache-redis",
      "status": "succeeded",
      "checks": [ ]
    }
  ]
}
```

| Summary Field | Description |
|---------------|-------------|
| `total` | Total number of repos in the batch |
| `succeeded` | Repos that copied and verified successfully |
| `failed` | Repos that encountered errors during copy |
| `skipped` | Repos skipped (e.g., already existed with `--skip-existing`) |
| `releases_skipped` | Repos whose releases were skipped due to rate-limit exhaustion (omitted when 0) |

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
  visibility: private  # Target repo visibility: private, public, or internal

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

In addition, all GitHub API calls automatically handle **rate limit responses**:
- **Primary rate limit (403):** When a request receives a `403` with `X-RateLimit-Remaining: 0`, vcopy sleeps until the `X-RateLimit-Reset` time (plus a 5-second buffer) and retries.
- **Secondary/abuse rate limit (403 + Retry-After):** When a request receives a `403` with a `Retry-After` header (GitHub's content creation throttle), vcopy sleeps for the specified duration (plus a 2-second buffer) and retries.
- **Secondary/abuse rate limit (429):** When a request receives a `429 Too Many Requests`, vcopy reads the `Retry-After` header and sleeps for that duration (plus a 2-second buffer). If no `Retry-After` header is present, it waits 60 seconds.

All three types retry up to 3 times per request, with a maximum single wait of 2 minutes. This applies to both authenticated and unauthenticated (`--public-source`) clients, so even with the 60 req/hr unauthenticated limit, operations will pause and resume rather than fail.

**Batch-level recovery:** In batch mode, if a secondary rate limit is hit during repo creation, vcopy waits for the full reset period (up to 10 minutes) and retries the failed repo automatically. The `--batch-delay` flag (default 3s) spaces out repo creations to prevent triggering secondary limits in the first place.

This handles transient network failures and GitHub API rate limits gracefully.

## GitHub Action

vcopy is available as a [reusable GitHub Action](https://github.com/marketplace) with all CLI capabilities. Use it directly in your workflows:

### Quick Start

```yaml
- uses: jpmicrosoft/vcopy@v1
  with:
    source-repo: source-org/my-repo
    target-org: target-org
    source-token: ${{ secrets.SOURCE_GITHUB_TOKEN }}
    target-token: ${{ secrets.TARGET_GITHUB_TOKEN }}
```

### Setup

1. **Reference the action** from any workflow: `uses: jpmicrosoft/vcopy@v1`
2. All CLI flags are available as action inputs (`force`, `code-only`, `lfs`, `all-metadata`, etc.)
3. The action runs in `--non-interactive` mode automatically

For full documentation, inputs/outputs reference, and example workflows, see **[action/README.md](action/README.md)**.

## Troubleshooting / FAQ

**The release binary is named `vcopy-windows-amd64.exe`, not `vcopy.exe`.**
Pre-built binaries include the platform and architecture in the filename. You can either run it directly (`./vcopy-windows-amd64.exe myorg/repo target-org`) or rename it to `vcopy.exe` (or `vcopy` on Linux/macOS) so the examples in this README work as-is. No installation step is required — it's a standalone executable.

**Authentication fails with "401 Unauthorized".**
Ensure your token has the `repo` scope. For public source repos, use `--public-source` to skip source authentication entirely.

**"Must have admin rights to Repository" when using `--force`.**
Your target token needs admin-level access to the target repo. For organization-owned repos, ensure the token has `repo` scope and the user is an org owner or has admin role on the repo.

**Verification shows WARN instead of PASS.**
WARN can occur for two reasons:
1. **Extra target content (additive mode):** The target contains branches, objects, or refs from prior copy runs that don't exist in the source. This is expected in additive mode and does not indicate data loss — all source content was verified as present.
2. **Rejected branches:** Some branches were rejected by the target during push — typically because of org rulesets or branch protection rules. vcopy automatically excludes rejected branches from verification so they don't cause false failures.

In both cases, WARN means all source content that was expected in the target was verified successfully.

**Verification fails on `refs/pull/*` refs.**
GitHub creates hidden `refs/pull/*` refs for pull requests. These are read-only and cannot be pushed. vcopy automatically excludes them during verification — if you see failures, ensure you're on the latest version.

**Windows SmartScreen warns "Windows protected your PC" when running the binary.**
This is expected for any unsigned executable downloaded from the internet. SmartScreen uses reputation-based filtering — new binaries with few downloads will trigger the warning. Two options:

- **Run anyway**: Click **"More info"** → **"Run anyway"**. The binary is safe — you can verify its integrity by comparing the SHA-256 checksum from the release's `checksums.txt` against the downloaded file:
  ```powershell
  (Get-FileHash vcopy-windows-amd64.exe -Algorithm SHA256).Hash
  ```
- **Build from source** (no SmartScreen warning): Building locally produces a binary that Windows trusts since it wasn't downloaded from the internet:
  ```powershell
  git clone https://github.com/jpmicrosoft/vcopy.git
  cd vcopy
  go build -o vcopy.exe ./cmd/vcopy
  ```

---

## Contributing

Contributions are welcome! Please open an issue or pull request on [GitHub](https://github.com/jpmicrosoft/vcopy).

## Author

**JP** — [github.com/jpmicrosoft](https://github.com/jpmicrosoft)
