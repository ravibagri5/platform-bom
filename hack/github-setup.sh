#!/usr/bin/env bash
# One-time GitHub setup for platform-bom: repository settings, milestones, the
# roadmap project board and seed issues. Safe to re-run.
#
# Run it after the repository exists and main has been pushed:
#   DRY_RUN=1 hack/github-setup.sh   # print what would happen
#   hack/github-setup.sh
set -euo pipefail

OWNER=${OWNER:-ravibagri5}
REPO=${REPO:-platform-bom}
SLUG="$OWNER/$REPO"
PROJECT_TITLE=${PROJECT_TITLE:-"platform-bom Roadmap"}
# The crossplane-mcp-server board, copied for its views and Status, Priority and Size fields.
TEMPLATE_PROJECT=${TEMPLATE_PROJECT:-1}
SEED_ISSUES=${SEED_ISSUES:-1}

run() {
  if [[ -n "${DRY_RUN:-}" ]]; then
    printf '+ %q' "$@"; echo
  else
    "$@"
  fi
}

step() { printf '\n== %s\n' "$*"; }

command -v gh >/dev/null || { echo "gh is required: https://cli.github.com" >&2; exit 1; }
command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }
gh repo view "$SLUG" >/dev/null || { echo "$SLUG does not exist yet; create and push it first" >&2; exit 1; }

step "Repository settings"
run gh repo edit "$SLUG" \
  --description "Your internal platform, as a versioned product: discover components across clusters, publish platform releases, track drift and upstream updates." \
  --enable-issues --enable-discussions --enable-projects \
  --enable-squash-merge --enable-merge-commit=false --enable-rebase-merge=false \
  --delete-branch-on-merge \
  --add-topic platform-engineering,kubernetes,internal-developer-platform,sbom,gitops,crossplane,inventory,golang
run gh api -X PUT "repos/$SLUG/private-vulnerability-reporting" --silent

step "Milestones"
milestones=(
  "v0.1 — Foundations|Resource model, discovery, catalog, releases, drift, upstream updates, CLI and UI."
  "v0.2 — The Git side of GitOps|Argo CD and Flux evidence sources, and release lint."
  "v0.3 — Lifecycle intelligence|End-of-life data, security advisories, compatibility rules and upgrade paths."
  "v0.4 — History|Inventory snapshots, what changed, and per-environment release timelines."
)
existing=$(gh api "repos/$SLUG/milestones?state=all&per_page=100" --jq '.[].title')
for m in "${milestones[@]}"; do
  title=${m%%|*}
  if grep -qxF "$title" <<<"$existing"; then
    echo "exists: $title"
  else
    run gh api "repos/$SLUG/milestones" -f title="$title" -f description="${m#*|}" --silent
  fi
done

step "Project board"
number=$(gh project list --owner "$OWNER" --format json \
  --jq ".projects[] | select(.title == \"$PROJECT_TITLE\") | .number" | head -1)
if [[ -z "$number" ]]; then
  if [[ -n "${DRY_RUN:-}" ]]; then
    run gh project copy "$TEMPLATE_PROJECT" --source-owner "$OWNER" --target-owner "$OWNER" --title "$PROJECT_TITLE"
    number="<new>"
  else
    number=$(gh project copy "$TEMPLATE_PROJECT" --source-owner "$OWNER" --target-owner "$OWNER" \
      --title "$PROJECT_TITLE" --format json --jq '.number')
    # The copied Theme and Area fields describe crossplane-mcp-server, so replace them.
    for field in Theme Area; do
      id=$(gh project field-list "$number" --owner "$OWNER" --format json \
        --jq ".fields[] | select(.name == \"$field\") | .id")
      [[ -n "$id" ]] && gh project field-delete --id "$id" >/dev/null
    done
    gh project field-create "$number" --owner "$OWNER" --name Theme --data-type SINGLE_SELECT \
      --single-select-options "Discover,Version,Advise,History,Present" >/dev/null
    gh project field-create "$number" --owner "$OWNER" --name Area --data-type SINGLE_SELECT \
      --single-select-options "api,catalog,discovery,upstream,analysis,release,server,cli,ui,deploy,build,ci" >/dev/null
    echo "created project #$number"
  fi
else
  echo "exists: project #$number"
fi
run gh project edit "$number" --owner "$OWNER" --visibility PUBLIC \
  --description "Roadmap and delivery board for platform-bom. Grouped by milestone (v0.1 to v0.4) and theme."
run gh project link "$number" --owner "$OWNER" --repo "$SLUG"

if [[ "$SEED_ISSUES" == "1" ]]; then
  step "Labels (from .github/labels.yml)"
  if [[ -n "${DRY_RUN:-}" ]]; then
    run gh workflow run labels.yaml --repo "$SLUG"
  else
    gh workflow run labels.yaml --repo "$SLUG"
    sleep 5
    run_id=$(gh run list --repo "$SLUG" --workflow labels.yaml --limit 1 --json databaseId --jq '.[0].databaseId')
    gh run watch "$run_id" --repo "$SLUG" --exit-status >/dev/null
    echo "labels synced"
  fi

  step "Seed issues from ROADMAP.md"
  # title|milestone|labels|body
  seeds=(
    "Discover Argo CD Applications as evidence|v0.2 — The Git side of GitOps|enhancement,area/discovery,theme/discover,priority/high|Read Application resources for target revision, sync status and images, so the matrix can compare the promised release, Git and the running version."
    "Discover Flux HelmReleases and Kustomizations as evidence|v0.2 — The Git side of GitOps|enhancement,area/discovery,theme/discover,priority/medium|The Flux equivalent of the Argo CD evidence source."
    "pbom release lint|v0.2 — The Git side of GitOps|enhancement,area/release,theme/version,priority/medium|Check a release file against the catalog before it merges: unknown components, versions that do not exist upstream."
    "End-of-life data from endoflife.date and managed Kubernetes support windows|v0.3 — Lifecycle intelligence|enhancement,area/upstream,theme/advise,priority/high|Make \"unsupported\" reflect EKS, AKS and GKE calendars as well as upstream support."
    "Security advisories for running versions|v0.3 — Lifecycle intelligence|enhancement,area/upstream,theme/advise,priority/medium|Show GitHub Security Advisories and OSV entries for the versions each environment runs."
    "Compatibility rules between components|v0.3 — Lifecycle intelligence|proposal,area/catalog,theme/advise,needs design|Declare in the catalog which versions of one component need which versions of another, for example Karpenter and Kubernetes."
    "Inventory history and what changed this week|v0.4 — History|proposal,area/analysis,theme/history,needs design|Keep discovery snapshots so the UI can show what changed and when each environment moved between releases."
  )
  open_titles=$(gh issue list --repo "$SLUG" --state all --limit 500 --json title --jq '.[].title')
  for s in "${seeds[@]}"; do
    IFS='|' read -r title milestone labels body <<<"$s"
    if grep -qxF "$title" <<<"$open_titles"; then
      echo "exists: $title"
      continue
    fi
    if [[ -n "${DRY_RUN:-}" ]]; then
      run gh issue create --repo "$SLUG" --title "$title" --milestone "$milestone" --label "$labels" --body "$body"
    else
      url=$(gh issue create --repo "$SLUG" --title "$title" --milestone "$milestone" --label "$labels" --body "$body")
      gh project item-add "$number" --owner "$OWNER" --url "$url" >/dev/null
      echo "created: $url"
    fi
  done
fi

step "Branch protection for main"
checks='["Verify","Lint","Test (ubuntu-latest)","Test (macos-latest)","Test (windows-latest)","Web UI","Build","Vulnerability scan","Container image","Check sign-off"]'
protection=$(jq -n --argjson checks "$checks" '{
  required_status_checks: {strict: true, contexts: $checks},
  enforce_admins: false,
  required_pull_request_reviews: {required_approving_review_count: 1, require_code_owner_reviews: true, dismiss_stale_reviews: true},
  restrictions: null,
  required_linear_history: true,
  allow_force_pushes: false,
  allow_deletions: false
}')
if [[ -n "${DRY_RUN:-}" ]]; then
  run gh api -X PUT "repos/$SLUG/branches/main/protection" --input -
else
  gh api -X PUT "repos/$SLUG/branches/main/protection" --input - --silent <<<"$protection"
fi

step "Manual steps"
cat <<EOF
The GitHub API cannot do these, so do them once in the browser:
  - Discussions categories (Announcements, Q&A, Ideas, Show and tell, Catalog, Contributors):
    https://github.com/$SLUG/discussions/categories
  - Board view: group by Milestone, slice by Theme, in the project's Board view:
    https://github.com/users/$OWNER/projects/$number
EOF
