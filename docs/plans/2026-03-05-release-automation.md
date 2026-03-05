# Release Automation Script Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create a bash script (`scripts/release.sh`) and Makefile wrapper that automates the full Reloader release process using `gh` CLI, replacing manual steps with interactive confirmations.

**Architecture:** Single bash script that orchestrates 7 phases sequentially: validate inputs, create release branch, trigger Init Release workflow + poll/merge its PR, create GitHub release (tag), create helm chart bump branch + PR, merge helm chart PR. Each destructive step prompts for `[y/N]` confirmation. A Makefile target `release-full` wraps the script.

**Tech Stack:** Bash, `gh` CLI, `sed`, `yq` (already in Makefile), `git`

---

### Task 1: Create the release script with input validation and helpers

**Files:**
- Create: `scripts/release.sh`

**Step 1: Create `scripts/release.sh` with argument parsing, validation, and helper functions**

```bash
#!/usr/bin/env bash
set -euo pipefail

REPO="stakater/Reloader"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info()    { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*" >&2; }

confirm() {
    local msg="$1"
    echo -en "${YELLOW}$msg [y/N]:${NC} "
    read -r answer
    [[ "$answer" =~ ^[Yy]$ ]]
}

usage() {
    cat <<EOF
Usage: $0 <APP_VERSION> <CHART_VERSION>

Automates the full Reloader release process.

Arguments:
  APP_VERSION    Application version without 'v' prefix (e.g. 1.5.0)
  CHART_VERSION  Helm chart version (e.g. 2.3.0)

Prerequisites:
  - gh CLI authenticated with repo access
  - git configured with push access to $REPO

Example:
  $0 1.5.0 2.3.0
EOF
    exit 1
}

# --- Input validation ---
[[ $# -ne 2 ]] && usage

APP_VERSION="$1"
CHART_VERSION="$2"

# Strip 'v' prefix if provided
APP_VERSION="${APP_VERSION#v}"
CHART_VERSION="${CHART_VERSION#v}"

# Validate semver format (simple check)
if ! [[ "$APP_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    error "APP_VERSION '$APP_VERSION' is not valid semver (expected X.Y.Z)"
    exit 1
fi

if ! [[ "$CHART_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    error "CHART_VERSION '$CHART_VERSION' is not valid semver (expected X.Y.Z)"
    exit 1
fi

# Check prerequisites
if ! command -v gh &> /dev/null; then
    error "gh CLI is not installed. Install from https://cli.github.com/"
    exit 1
fi

if ! gh auth status &> /dev/null; then
    error "gh CLI is not authenticated. Run 'gh auth login' first."
    exit 1
fi

RELEASE_BRANCH="release-v${APP_VERSION}"
TAG="v${APP_VERSION}"

info "Release plan:"
info "  App version:    $APP_VERSION (tag: $TAG)"
info "  Chart version:  $CHART_VERSION"
info "  Release branch: $RELEASE_BRANCH"
echo ""
```

**Step 2: Make it executable and verify it runs**

Run: `chmod +x scripts/release.sh && ./scripts/release.sh`
Expected: Usage message printed, exit 1

Run: `./scripts/release.sh 1.5.0 2.3.0`
Expected: Prints release plan, then continues (will fail at next phase since we haven't written it yet)

**Step 3: Commit**

```bash
git add scripts/release.sh
git commit -m "feat: add release script with input validation and helpers"
```

---

### Task 2: Add Phase 1 — Create release branch

**Files:**
- Modify: `scripts/release.sh` (append after validation block)

**Step 1: Append Phase 1 to the script**

```bash
# =============================================================================
# Phase 1: Create release branch
# =============================================================================
info "Phase 1: Create release branch '$RELEASE_BRANCH' from master"

if git ls-remote --heads origin "$RELEASE_BRANCH" | grep -q "$RELEASE_BRANCH"; then
    warn "Branch '$RELEASE_BRANCH' already exists on remote."
    if ! confirm "Continue using existing branch?"; then
        error "Aborted."
        exit 1
    fi
else
    if ! confirm "Create and push branch '$RELEASE_BRANCH' from master?"; then
        error "Aborted."
        exit 1
    fi
    git fetch origin master
    git push origin origin/master:refs/heads/"$RELEASE_BRANCH"
    info "Branch '$RELEASE_BRANCH' created and pushed."
fi
echo ""
```

**Step 2: Commit**

```bash
git add scripts/release.sh
git commit -m "feat(release): add phase 1 - create release branch"
```

---

### Task 3: Add Phase 2 — Trigger Init Release workflow and poll/merge its PR

**Files:**
- Modify: `scripts/release.sh` (append)

**Step 1: Append Phase 2**

The Init Release workflow creates a PR from a branch prefixed with `update-version` targeting the release branch. We trigger it, poll for the PR, then merge it.

```bash
# =============================================================================
# Phase 2: Trigger Init Release workflow and merge its PR
# =============================================================================
info "Phase 2: Trigger Init Release workflow"

if ! confirm "Trigger 'Init Release' workflow for branch '$RELEASE_BRANCH' with version '$APP_VERSION'?"; then
    error "Aborted."
    exit 1
fi

gh workflow run init-branch-release.yaml \
    --repo "$REPO" \
    -f TARGET_BRANCH="$RELEASE_BRANCH" \
    -f TARGET_VERSION="$APP_VERSION"

info "Workflow triggered. Waiting for version bump PR to be created..."

# Poll for the PR (created by the workflow targeting the release branch)
MAX_ATTEMPTS=30
SLEEP_INTERVAL=10
PR_NUMBER=""

for i in $(seq 1 $MAX_ATTEMPTS); do
    PR_NUMBER=$(gh pr list \
        --repo "$REPO" \
        --base "$RELEASE_BRANCH" \
        --search "Bump version to $APP_VERSION" \
        --json number \
        --jq '.[0].number // empty' 2>/dev/null || true)

    if [[ -n "$PR_NUMBER" ]]; then
        info "Found PR #$PR_NUMBER"
        break
    fi
    echo -n "."
    sleep "$SLEEP_INTERVAL"
done

if [[ -z "$PR_NUMBER" ]]; then
    error "Timed out waiting for Init Release PR. Check workflow status at:"
    error "  https://github.com/$REPO/actions/workflows/init-branch-release.yaml"
    exit 1
fi

info "PR: https://github.com/$REPO/pull/$PR_NUMBER"

if ! confirm "Merge PR #$PR_NUMBER (version bump to $APP_VERSION)?"; then
    error "Aborted. PR is still open: https://github.com/$REPO/pull/$PR_NUMBER"
    exit 1
fi

gh pr merge "$PR_NUMBER" --repo "$REPO" --merge
info "PR #$PR_NUMBER merged."
echo ""
```

**Step 2: Commit**

```bash
git add scripts/release.sh
git commit -m "feat(release): add phase 2 - trigger init release and merge PR"
```

---

### Task 4: Add Phase 3 — Create GitHub release

**Files:**
- Modify: `scripts/release.sh` (append)

**Step 1: Append Phase 3**

```bash
# =============================================================================
# Phase 3: Create GitHub release
# =============================================================================
info "Phase 3: Create GitHub release '$TAG' targeting '$RELEASE_BRANCH'"
info "This will trigger the release workflow (Docker image builds, GoReleaser)."

if ! confirm "Create GitHub release '$TAG'?"; then
    error "Aborted."
    exit 1
fi

gh release create "$TAG" \
    --repo "$REPO" \
    --target "$RELEASE_BRANCH" \
    --title "Release $TAG" \
    --generate-notes

info "GitHub release created: https://github.com/$REPO/releases/tag/$TAG"
info "Release workflow will run in the background."
echo ""
```

**Step 2: Commit**

```bash
git add scripts/release.sh
git commit -m "feat(release): add phase 3 - create GitHub release"
```

---

### Task 5: Add Phase 4 — Create helm chart bump PR and merge it

**Files:**
- Modify: `scripts/release.sh` (append)

**Step 1: Append Phase 4**

This phase creates a branch from master, bumps chart version + appVersion + image tag, pushes, creates a PR with the `release/helm-chart` label, and merges it.

```bash
# =============================================================================
# Phase 4: Bump Helm chart and create PR
# =============================================================================
info "Phase 4: Bump Helm chart version to $CHART_VERSION (appVersion: v$APP_VERSION)"

HELM_BRANCH="release-helm-chart-v${CHART_VERSION}"

if ! confirm "Create branch '$HELM_BRANCH', bump chart files, and open PR with 'release/helm-chart' label?"; then
    error "Aborted."
    exit 1
fi

# Create branch from latest master
git fetch origin master
git checkout -b "$HELM_BRANCH" origin/master

# Bump Chart.yaml: version and appVersion
CHART_FILE="deployments/kubernetes/chart/reloader/Chart.yaml"
sed -i "s/^version:.*/version: ${CHART_VERSION}/" "$CHART_FILE"
sed -i "s/^appVersion:.*/appVersion: v${APP_VERSION}/" "$CHART_FILE"

# Bump values.yaml: image.tag
VALUES_FILE="deployments/kubernetes/chart/reloader/values.yaml"
sed -i "s/^\(  tag:\).*/\1 v${APP_VERSION}/" "$VALUES_FILE"

# Show changes for review
info "Changes:"
git diff

git add "$CHART_FILE" "$VALUES_FILE"
git commit -m "Bump helm chart to ${CHART_VERSION} and appVersion to v${APP_VERSION}"
git push origin "$HELM_BRANCH"

HELM_PR_URL=$(gh pr create \
    --repo "$REPO" \
    --base master \
    --head "$HELM_BRANCH" \
    --title "Bump Helm chart to ${CHART_VERSION} (appVersion v${APP_VERSION})" \
    --body "Bump Helm chart version to ${CHART_VERSION} and appVersion to v${APP_VERSION}." \
    --label "release/helm-chart")

HELM_PR_NUMBER=$(echo "$HELM_PR_URL" | grep -o '[0-9]*$')
info "Helm chart PR created: $HELM_PR_URL"

if ! confirm "Merge Helm chart PR #$HELM_PR_NUMBER?"; then
    error "Aborted. PR is still open: $HELM_PR_URL"
    exit 1
fi

gh pr merge "$HELM_PR_NUMBER" --repo "$REPO" --merge
info "Helm chart PR #$HELM_PR_NUMBER merged."

# Return to previous branch
git checkout -

echo ""
info "============================================="
info "Release $TAG complete!"
info "============================================="
info ""
info "Summary:"
info "  - Release branch: $RELEASE_BRANCH"
info "  - GitHub release: https://github.com/$REPO/releases/tag/$TAG"
info "  - Helm chart: $CHART_VERSION (appVersion: v$APP_VERSION)"
info ""
info "The release workflow is running in the background."
info "Monitor at: https://github.com/$REPO/actions"
```

**Step 2: Commit**

```bash
git add scripts/release.sh
git commit -m "feat(release): add phase 4 - helm chart bump PR and merge"
```

---

### Task 6: Add Makefile wrapper

**Files:**
- Modify: `Makefile:157` (append after `update-manifests-version` target, before the yq section)

**Step 1: Add `release-full` target to Makefile**

Add this block after the `update-manifests-version` target (around line 159):

```makefile
.PHONY: release-full
release-full: ## Run the full release process (interactive)
ifndef APP_VERSION
	$(error APP_VERSION is required. Usage: make release-full APP_VERSION=X.Y.Z CHART_VERSION=A.B.C)
endif
ifndef CHART_VERSION
	$(error CHART_VERSION is required. Usage: make release-full APP_VERSION=X.Y.Z CHART_VERSION=A.B.C)
endif
	./scripts/release.sh $(APP_VERSION) $(CHART_VERSION)
```

**Step 2: Verify Makefile target works**

Run: `make release-full`
Expected: Error about APP_VERSION being required

Run: `make release-full APP_VERSION=1.5.0 CHART_VERSION=2.3.0`
Expected: Prints release plan, begins interactive flow

**Step 3: Commit**

```bash
git add Makefile
git commit -m "feat: add release-full Makefile target"
```

---

### Task 7: Final verification and combined commit

**Step 1: Verify complete script syntax**

Run: `bash -n scripts/release.sh`
Expected: No syntax errors

**Step 2: Verify help output**

Run: `./scripts/release.sh`
Expected: Usage message with examples

**Step 3: Verify invalid input handling**

Run: `./scripts/release.sh bad-version also-bad`
Expected: Error about invalid semver

Run: `./scripts/release.sh v1.5.0 2.3.0`
Expected: Strips the 'v' prefix, shows plan with `1.5.0`
