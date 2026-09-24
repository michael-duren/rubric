#!/usr/bin/env bash
# Print which CI areas a change touches, as key=value lines for $GITHUB_OUTPUT.
#
# Usage: changed-areas.sh BASE_SHA HEAD_SHA
# An empty, all-zero, or unknown BASE (new branch, first push, manual run) selects every area.
set -euo pipefail

base=${1:-}
head=${2:-HEAD}

all=false
if [ -z "$base" ] || [ "$base" = "0000000000000000000000000000000000000000" ] || ! git cat-file -e "$base^{commit}" 2>/dev/null; then
	all=true
	files=""
else
	files=$(git diff --name-only "$base" "$head")
fi

matches() {
	$all || printf '%s\n' "$files" | grep -Eq "$1"
}

go_paths='(\.go$|^go\.(mod|sum)$|^cmd/|^internal/|^\.golangci\.yml$|^docs/cli-init\.md$|^\.github/)'
script_paths='(^scripts/|^\.github/)'

if matches "$go_paths"; then echo "go=true"; else echo "go=false"; fi
if matches "$script_paths"; then echo "scripts=true"; else echo "scripts=false"; fi
