# `rubric update`

`rubric update [directory]` shows the Rubric features recorded in a repository's
`rubric.yaml` and changes them. The directory must already have a `rubric.yaml`
from [`rubric init`](cli-init.md); without one, update exits with status 2.

## Features

| Menu | Feature | Flag | Files |
| --- | --- | --- | --- |
| Skills | Rubric skills | `--skills=rubric` | `.agents/skills/rubric-{workflow,testing,style}/SKILL.md` |
| Skills | pstack skills | `--skills=pstack` | Vendored pstack workflows in `.agents/skills/` (`poteto-mode`, `tdd`, …) |
| Skills | pstack principles | `--skills=principles` | `.agents/skills/principle-*` and `.agents/skills/THIRD_PARTY_NOTICES.md` |
| Skills | pstack agents | `--skills=agents` | Subagents in `.agents/agents/` |
| Tooling | Linting | `--lint` | `.golangci.yml`, `.rubric/style/`, and `.rubric/check.sh` |
| Tooling | Makefile | `--makefile` | `Makefile` and `.rubric/check.sh` |
| Tooling | GitHub Actions | `--actions` | `.github/workflows/ci.yml` and `.rubric/check.sh` |

pstack skills link to the principle skills, and the agents load pstack skills, so
`pstack` requires `principles` and `agents` requires `pstack`. The menu turns these on
and off together. On the command line, a group without the groups it needs is an error.

`rubric.yaml` stores skills as a list, for example `skills: [rubric, principles]`.
Older files with `skills: true` or `skills: false` still load as all groups or none,
and are rewritten as a list the next time Rubric saves them.

## Checking what is active

```text
rubric update --list
rubric update --list --format json
```

`--list` prints each feature as on or off, the detected application features, and
any file changes pending between `rubric.yaml` and the repository. For example, a
deleted `Makefile` shows up as a pending `create`. It never writes anything.

## Changing features

In a terminal, `rubric update` opens a menu with a **Skills** submenu and a **Tooling**
submenu. Move with the arrow keys, press Enter to open a submenu and Space to toggle,
and press Esc to go back. Changed toggles are marked `*`. Press `s` to save. You then
reach the same Review and Apply steps as `rubric init`. Esc on the top menu quits: with
no changes it exits 0, and with unsaved changes it cancels with status 130.

Without a terminal, or with `--non-interactive`, `--format json`, or `--dry-run`, the
flags are the changes:

```text
rubric update --skills=rubric,pstack,principles --lint --makefile=false
rubric update --skills=none --dry-run --format json
```

Saving reconciles the repository with the new settings:

- Files for features turned on are created. Files that still apply are refreshed while
  they still match what Rubric wrote.
- Files for features turned off are deleted if you have not edited them since Rubric
  wrote them. Directories left empty are removed.
- An edited file for a feature turned off is a conflict. In the menu, select it on the
  Review step and press `r` to delete it or `s` to keep it; a kept file is no longer
  managed by Rubric. Unattended runs stop with status 2 until you delete the file or
  restore its content.
- `AGENTS.md` guidance and `rubric.yaml` are updated to match.

Deletes are rechecked right before they happen and restored if the apply fails or is
cancelled, just like writes. The report lists them with state `delete`, and the JSON
`result.deleted` field names every deleted path. Other flags, the JSON report, and exit
statuses are the same as for [`rubric init`](cli-init.md).
