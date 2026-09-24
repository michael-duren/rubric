#!/usr/bin/env bash
# Vendor pstack's skills (https://github.com/cursor/plugins/tree/main/pstack) into this repository.
#
# Copies the skills relevant to Go projects from a pinned upstream commit, rewrites Cursor-specific
# skill paths to .agents/skills, and writes a single MIT attribution notice. The same files are
# written to this repository's .agents/ directory and to the renderer's embedded templates, which
# `rubric init --skills` installs into generated projects.
#
# Usage: scripts/vendor-pstack.sh [COMMIT]
set -euo pipefail

commit=${1:-fadd23794c0075468eb8964b0fd93e06e09486ad}
excluded_skills=(typescript-best-practices make-bot-ui setup-pstack)
repo_root=$(cd "$(dirname "$0")/.." && pwd)
destinations=("$repo_root/.agents" "$repo_root/internal/render/templates/pstack")

command -v gh >/dev/null || { echo "gh CLI is required" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
gh api "repos/cursor/plugins/tarball/$commit" > "$work/plugins.tgz"
mkdir "$work/src"
tar -xzf "$work/plugins.tgz" -C "$work/src" --strip-components=1 --wildcards '*/pstack/*'
src="$work/src/pstack"

stage="$work/stage"
mkdir -p "$stage/skills" "$stage/agents"
for dir in "$src"/skills/*/; do
	name=$(basename "$dir")
	for skip in "${excluded_skills[@]}"; do
		[ "$name" = "$skip" ] && continue 2
	done
	cp -R "$dir" "$stage/skills/$name"
done
cp "$src/agents/comment-sicko.md" "$src/agents/poteto-agent.md" "$stage/agents/"
find "$stage" -type f \( -name '*.test.ts' -o -name '*.test-helper.ts' -o -name '*.compile.ts' \) -delete

while IFS= read -r -d '' file; do
	if grep -Iq . "$file"; then
		sed -i -e 's#\.cursor/skills/#.agents/skills/#g' -e 's#pstack/skills/#.agents/skills/#g' "$file"
	fi
done < <(find "$stage" -type f -print0)

license=$(sed -n '1,/^SOFTWARE\.$/p' "$src/LICENSE")
tick='`'
excluded_list=""
for skip in "${excluded_skills[@]}"; do
	excluded_list+="${excluded_list:+, }${tick}${skip}${tick}"
done
cat > "$stage/skills/THIRD_PARTY_NOTICES.md" <<EOF
# Third-party notices

The skills in this directory other than \`rubric-*\`, and the agents in \`.agents/agents/\`, come from
[pstack](https://github.com/cursor/plugins/tree/main/pstack) at commit \`$commit\`.

Changes from upstream:

- Skill paths \`.cursor/skills/\` and \`pstack/skills/\` were rewritten to \`.agents/skills/\`.
- Not included: ${excluded_list}, pstack's docs, assets, automations, and the test files of \`poteto-mode/scripts\`.

$license
EOF

for dest in "${destinations[@]}"; do
	mkdir -p "$dest/skills" "$dest/agents"
	for dir in "$stage"/skills/*/; do
		rm -rf "$dest/skills/$(basename "$dir")"
	done
	cp -R "$stage"/skills/. "$dest/skills/"
	cp "$stage"/agents/*.md "$dest/agents/"
done

echo "vendored pstack $commit: $(find "$stage/skills" -name SKILL.md | wc -l) skills, $(find "$stage" -type f | wc -l) files"
