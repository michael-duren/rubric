#!/usr/bin/env bash
# Configure default-branch protection for a GitHub repository with the gh CLI.
#
# What it sets up (idempotent; rerunning updates the same ruleset):
#   - A repository ruleset on the default branch that:
#       * requires pull requests with approving reviews (stale approvals dismissed,
#         review threads resolved, code-owner review optional),
#       * requires the CI status checks to pass on an up-to-date branch,
#       * blocks force pushes and branch deletion.
#   - Repository admins bypass the ruleset ("exempt list"). GitHub rulesets cannot
#     list individual users on personal repositories, so the owner is exempted
#     through the admin role; on a personal repo the owner is the only admin.
#   - Pull requests merge by squash only, and head branches are deleted after merge.
#
# Usage:
#   scripts/setup-branch-protection.sh [--repo OWNER/NAME] [--branch NAME]
#       [--approvals N] [--check NAME ...] [--code-owners] [--dry-run]
#
# Defaults: current repository, its default branch, 1 approval, and the single "CI gate (required)"
# check from .github/workflows/ci.yml, which passes when path-filtered jobs pass or are skipped.
set -euo pipefail

RULESET_NAME="default-branch-protection"
ACTIONS_APP_ID=15368
ADMIN_ROLE_ID=5

repo=""
branch=""
approvals=1
code_owners=false
dry_run=false
checks=()

usage() {
	sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
	exit "${1:-0}"
}

while [ $# -gt 0 ]; do
	case "$1" in
	--repo) repo=${2:?--repo needs OWNER/NAME}; shift 2 ;;
	--branch) branch=${2:?--branch needs a name}; shift 2 ;;
	--approvals) approvals=${2:?--approvals needs a number}; shift 2 ;;
	--check) checks+=("${2:?--check needs a check name}"); shift 2 ;;
	--code-owners) code_owners=true; shift ;;
	--dry-run) dry_run=true; shift ;;
	-h | --help) usage 0 ;;
	*) printf 'unknown argument: %s\n\n' "$1" >&2; usage 2 ;;
	esac
done

command -v gh >/dev/null || { echo "gh CLI is required: https://cli.github.com" >&2; exit 1; }
gh auth status >/dev/null 2>&1 || { echo "run 'gh auth login' first" >&2; exit 1; }
case "$approvals" in '' | *[!0-9]*) echo "--approvals must be a non-negative integer" >&2; exit 2 ;; esac

if [ -z "$repo" ]; then
	repo=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
fi
if [ -z "$branch" ]; then
	branch=$(gh repo view "$repo" --json defaultBranchRef --jq .defaultBranchRef.name)
fi
if [ ${#checks[@]} -eq 0 ]; then
	checks=("CI gate (required)")
fi

permission=$(gh repo view "$repo" --json viewerPermission --jq .viewerPermission)
if [ "$permission" != "ADMIN" ]; then
	echo "you need admin permission on $repo (you have $permission)" >&2
	exit 1
fi

status_checks=$(printf '%s\n' "${checks[@]}" |
	jq -R --argjson app "$ACTIONS_APP_ID" '{context: ., integration_id: $app}' | jq -s .)

payload=$(jq -n \
	--arg name "$RULESET_NAME" \
	--arg ref "refs/heads/$branch" \
	--argjson approvals "$approvals" \
	--argjson code_owners "$code_owners" \
	--argjson admin "$ADMIN_ROLE_ID" \
	--argjson checks "$status_checks" \
	'{
		name: $name,
		target: "branch",
		enforcement: "active",
		conditions: {ref_name: {include: [$ref], exclude: []}},
		bypass_actors: [{actor_id: $admin, actor_type: "RepositoryRole", bypass_mode: "always"}],
		rules: [
			{type: "deletion"},
			{type: "non_fast_forward"},
			{type: "pull_request", parameters: {
				required_approving_review_count: $approvals,
				dismiss_stale_reviews_on_push: true,
				require_code_owner_review: $code_owners,
				require_last_push_approval: false,
				required_review_thread_resolution: true,
				allowed_merge_methods: ["squash"]
			}},
			{type: "required_status_checks", parameters: {
				strict_required_status_checks_policy: true,
				do_not_enforce_on_create: false,
				required_status_checks: $checks
			}}
		]
	}')

existing=$(gh api "repos/$repo/rulesets" --paginate --jq ".[] | select(.name == \"$RULESET_NAME\") | .id" | head -n 1)

echo "Repository:  $repo"
echo "Branch:      $branch"
echo "Approvals:   $approvals (code owners: $code_owners)"
echo "Checks:      $(printf '%s, ' "${checks[@]}" | sed 's/, $//')"
echo "Exempt:      repository admins (bypass: always)"
if [ -n "$existing" ]; then
	echo "Ruleset:     update #$existing"
else
	echo "Ruleset:     create"
fi
echo "Merging:     squash only; head branches auto-deleted after merge"

if $dry_run; then
	echo
	echo "Dry run; ruleset payload:"
	echo "$payload" | jq .
	exit 0
fi

if [ -n "$existing" ]; then
	echo "$payload" | gh api --method PUT "repos/$repo/rulesets/$existing" --input - --jq '"updated ruleset #\(.id)"'
else
	echo "$payload" | gh api --method POST "repos/$repo/rulesets" --input - --jq '"created ruleset #\(.id)"'
fi

gh api --method PATCH "repos/$repo" -F delete_branch_on_merge=true -F allow_squash_merge=true -F allow_merge_commit=false -F allow_rebase_merge=false \
	--jq '"squash=\(.allow_squash_merge) merge_commit=\(.allow_merge_commit) rebase=\(.allow_rebase_merge) delete_branch_on_merge=\(.delete_branch_on_merge)"'
