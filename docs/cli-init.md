# `rubric init`

`rubric init [directory]` creates a Go project, or adds Rubric configuration,
agent guidance, and optional tooling to an existing Go module. The default
directory is the current directory.

## Install

Rubric is not published as a release yet. From a checkout of this repository:

```text
go install ./cmd/rubric            # installs rubric into $(go env GOPATH)/bin
go build -o ./bin/rubric ./cmd/rubric   # local development build
```

## Interactive and unattended use

In a terminal, `rubric init` opens a five-step wizard: Target, Application,
Tooling, Review, and Apply. Flags you pass become the wizard's starting answers.
Nothing is written until you press Enter on the Review step; Ctrl+C before that
leaves the directory unchanged and exits with status 130.

The wizard is skipped, and nothing prompts, when any of these hold:
standard input or output is not a terminal, `--non-interactive` is set,
`--format json` is set, or `--dry-run` is set.

## Flags

| Flag | Values | Meaning |
| --- | --- | --- |
| `--mode` | `auto` (default), `new`, `existing` | Detect the target or force a mode. A nonempty directory without `go.mod` needs `--mode new`. |
| `--module` | module path | Required for new projects. Plain local names such as `app` are accepted. |
| `--name` | text | Display name; defaults to the last module path element. |
| `--description` | text | Project description, stored as literal text. |
| `--starter` | `runnable` (default), `module` | Used when no HTTP, CLI, or TUI executable is selected. `runnable` adds `cmd/<module-name>/main.go` (it opens the store or loads settings when those are selected); `module` generates no Go files. |
| `--http` | `none`, `nethttp`, `chi` | HTTP server in `cmd/server` and `internal/httpserver`. |
| `--database` | `none`, `sqlite`, `postgres` | Database support in `internal/store`. |
| `--access` | `sql` (default with a database), `sqlc` | Handwritten `database/sql` or sqlc-generated queries. |
| `--cli` | `none`, `flag`, `cobra` | Command-line executable in `cmd/cli` and `internal/cli`. |
| `--tui` | `none`, `bubbletea` | Terminal UI in `cmd/tui` and `internal/tui`. |
| `--app-config` | `stdlib` (default), `viper` | Runtime settings in `internal/config`. Viper does not require Cobra. |
| `--web` | `none`, `htmx` | Server-rendered UI in `internal/web`: templ views, htmx requests, and Alpine.js behavior. Requires `--http`. |
| `--e2e` | `none`, `playwright` | Playwright browser tests in `e2e/` (build tag `e2e`). Requires `--web htmx`. |
| `--skills` | `all`, `none`, or groups | Agent skill groups, comma-separated: `rubric` (`.agents/skills/rubric-*/SKILL.md`), `pstack` (the vendored [pstack](https://github.com/cursor/plugins/tree/main/pstack) skills such as `poteto-mode`, `tdd`, and `create-verification-skill`), `principles` (the `principle-*` skills with the MIT notice in `.agents/skills/THIRD_PARTY_NOTICES.md`), and `agents` (subagents in `.agents/agents/`). `pstack` requires `principles`; `agents` requires `pstack`. `--skills` alone means `all`; pass a list with `=`, as in `--skills=rubric,pstack,principles`. |
| `--lint` | boolean | Add `.golangci.yml`, the comment analyzer in `.rubric/style`, and a `lint` check. |
| `--makefile` | boolean | Add a `Makefile` whose targets call `.rubric/check.sh`. |
| `--actions` | boolean | Add `.github/workflows/ci.yml`. It uses Make only when the Makefile is enabled. |
| `--entry-point` | JSON, repeatable | Existing entry point, for example `{"name":"api","dir":"cmd/api"}`. |
| `--command` | JSON, repeatable | Command as an argument list, for example `{"name":"test","argv":["go","test","./..."]}`. Never parsed by a shell. |
| `--clear-entry-points`, `--clear-commands` | boolean | Record an explicitly empty list. |
| `--config` | path | Read a Rubric configuration document as input. |
| `--non-interactive` | boolean | Never prompt or start the wizard. |
| `--dry-run` | boolean | Validate and print the plan without creating directories or writing files. |
| `--format` | `text` (default), `json` | Report format. JSON implies `--non-interactive`. |

Tooling toggles default to off. Explicit `--lint=false`, `--http none`, and
empty lists are kept; they are never replaced by defaults.

## Precedence

Explicit flags override `--config` input, which overrides the target's
`rubric.yaml`. For existing projects, detection fills facts that none of those
supply. Documented defaults apply last. `go.mod` is always the source of the
module path and Go version for existing projects; Rubric never rewrites it.

## Examples

The default starter with a `cmd/my-app/main.go` executable, then a module-only starter with no Go files:

```sh
rubric init my-app --module example.com/my-app
rubric init my-app --module example.com/my-app --starter module
```

A service with every executable, SQLite through sqlc, Viper settings, and tooling:

```sh
rubric init my-app --module example.com/my-app --http chi --cli cobra --tui bubbletea --database sqlite --access sqlc --app-config viper --lint --makefile --actions --skills
```

An htmx, Alpine.js, and templ web UI with Playwright browser tests:

```sh
rubric init my-app --module example.com/my-app --http chi --web htmx --e2e playwright --makefile
```

Library-only PostgreSQL support without an HTTP server:

```sh
rubric init my-app --module example.com/my-app --database postgres --access sql
```

Headless preview with a JSON report, then input from a configuration file:

```sh
rubric init my-app --module example.com/my-app --http nethttp --dry-run --format json
rubric init my-app --config input.yaml --non-interactive
```

Adding Rubric to an existing module, with corrected entry points:

```sh
rubric init legacy-app --lint --skills
rubric init legacy-app --entry-point '{"name":"app","dir":"."}' --format json
```

## Output

Every new project gets `go.mod`, `README.md`, `rubric.yaml`, `AGENTS.md`,
`.rubric/style.md`, and `.rubric/manifest.json`. Existing projects get
`rubric.yaml`, a managed section in `AGENTS.md`, `.rubric/style.md`, and the
manifest; application code, tests, and `go.mod` are left alone.

The JSON report is one document on standard output with `status`, `mode`,
`config`, `actions`, `conflicts`, `diagnostics`, `next_commands`,
and `result`. Rendered file contents and environment values are never included.

Exit status: `0` success or a conflict-free dry run, `2` invalid input or
unresolved conflicts, `1` operational failure, `130` cancelled.

## After generation

Generation does not download dependencies or run your tests. Run the setup
command from the report, then build and test:

```text
go mod tidy
go build ./...
go test ./...
```

A module-only project has no packages yet; `sh .rubric/check.sh test` (or
`make test`) reports that and starts checking automatically once code exists.
With linting enabled, `sh .rubric/check.sh lint` runs `golangci-lint fmt --diff`,
`golangci-lint run`, and `go run ./.rubric/style .`, using pinned versions through
`go run`; nothing needs to be installed globally.

sqlc projects regenerate queries with `sh .rubric/check.sh generate` (or
`make generate`), which runs the pinned sqlc through `go run`.

## Web UI and browser tests

`--web htmx` adds `internal/web`: `views.templ` and its generated `views_templ.go`, handlers
for the page and an htmx fragment, and pinned `htmx.min.js` and `alpine.min.js` served from
`/static/` (licenses in `internal/web/static/THIRD_PARTY_LICENSES.md`). Nothing is loaded from
a CDN. After editing `views.templ`, run the `generate-templ` command
(`go run github.com/a-h/templ/cmd/templ@v0.3.1020 generate`). Unit tests exercise the
handlers and rendered HTML without a browser.

`--e2e playwright` adds `e2e/ui_test.go`, which drives Chromium through
`github.com/mxschmitt/playwright-go`. Browser tests never run in the default `go test ./...`;
install the browser once, then run them explicitly:

```text
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install chromium   # setup-e2e
go test -tags e2e ./e2e/...                                                            # test-e2e
```

With GitHub Actions enabled, the workflow installs Chromium with its system dependencies,
checks that `views_templ.go` is up to date, and runs the browser tests.

## Runtime settings and PostgreSQL

Generated executables read `APP_HTTP_ADDR` (default `:8080`) and
`APP_DATABASE_URL`, plus `APP_CONFIG_FILE` for Viper. PostgreSQL projects
require `APP_DATABASE_URL`; credentials never go into source or `rubric.yaml`.
Apply `sql/schema.sql` to your database yourself.

Default `go test ./...` never needs a database. PostgreSQL integration tests run
only when enabled:

```text
TEST_DATABASE_URL=postgres://postgres@127.0.0.1:5432/rubric_test?sslmode=disable \
  go test -tags integration ./internal/store/...
```

With GitHub Actions enabled, the workflow starts a disposable `postgres:17.6`
service and runs that step for you.

## Conflicts, ownership, and recovery

The review lists every path as `create`, `update`, `unchanged`, `conflict`, or
`skip`. Rubric records hashes of content it wrote in `.rubric/manifest.json`
and refreshes a managed file or the `AGENTS.md` section between
`<!-- rubric:begin -->` and `<!-- rubric:end -->` only while it still matches.
Text outside those markers is always preserved.

Unattended runs stop before writing anything when a conflict exists and list
it in the report; edit, move, or delete the file, then rerun. The wizard lets
you replace or skip each conflict; skipping optional tooling turns that tooling
off so guidance never mentions it. `rubric.yaml` and `AGENTS.md` cannot be skipped.

Files are staged and each target is rechecked immediately before replacement.
If a write fails or you cancel during apply, Rubric restores the files it wrote
when they have not changed since; paths it cannot safely restore are listed in
the error. A file Rubric wrote but no longer generates, because its feature was
turned off, is deleted while it still matches what Rubric wrote, and directories
left empty are removed. If you edited it, it is a conflict: the wizard lets you
delete it (`r`) or keep it (`s`, after which Rubric stops managing it); unattended
runs stop until you delete or restore it. Rerunning an unchanged project is a no-op.
To change features later, use [`rubric update`](cli-update.md).
