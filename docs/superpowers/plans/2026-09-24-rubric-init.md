# Rubric Init v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the approved `rubric init` wizard and unattended generator for new and existing Go projects, with project-aware guidance and mandatory tests.

**Architecture:** A typed configuration model feeds static repository detection, bundled template rendering, a reviewed file plan, and a conflict-aware writer. Cobra and Bubble Tea are adapters around the same initialization service. Generated applications keep executable wiring in `cmd/` and tested behavior in `internal/`.

**Tech Stack:** Go 1.26.7, Bubble Tea v2, Cobra, go-yaml v3, Go AST/module parsing, `text/template`, the standard Go test package, and the approved generated-library catalog.

**Spec:** [Approved design](../specs/2026-09-24-rubric-init-design.md). Read both documents before execution.

## Global Constraints

- "Implement `rubric init` with a Bubble Tea wizard and noninteractive operation."
- "Offer both a module-only starter and a minimal runnable starter."
- "Use curated library choices, with all application capabilities optional."
- "Explicit disabled values and empty lists must not be replaced by defaults."
- "Selecting JSON implies noninteractive operation."
- Initial supported Go baseline: `1.26.7`; preserve existing `go.mod` declarations.
- Generation and dry runs themselves do not execute package installation.
- "Disabling Makefile, linting, skills, or CI does not remove generated tests."
- Generated test exemptions: files named `main.go` and files beneath any directory named `cmd`; keep substantive behavior outside them.
- All Rubric behavior needs tests, including CLI behavior exercised through integration tests.
- "A declaration's documentation comment has at most two physical lines, each containing at most 150 Unicode characters, including its comment delimiter."
- No explanatory inline/trailing Go comments; documented compiler/tool directives and third-party generated-code exceptions apply.
- Exit statuses: success `0`, invalid input/conflict `2`, operational failure `1`, cancellation `130`.
- No application scaffolding or dependency upgrades in existing repositories; no automatic obsolete-file deletion.
- No `rubric validate`, `rubric make`, `rubric paths`, `rubric perf`, external template plugins, or multi-module workspace orchestration in this plan.
- Never add a co-author trailer to commits (`AGENTS.md`).

## Review Focus

1. Explicit `false`, `none`, and empty command/entry-point lists must override saved values; YAML comments must survive edits. Test in Task 1.
2. Paths containing spaces, `$`, backticks, or quotes must remain literal in source, commands, and previews; no shell expansion or traversal. Test in Tasks 3, 6, and 15.
3. A file or symlink changed between preview and apply must stop the write; rollback must not erase an intervening user edit. Test in Tasks 5–6.
4. Rerunning init after a generated project has become an existing project must not rewrite application code or silently restore removed components. Test in Tasks 2, 7, and 17.
5. Module-only projects, redirected terminals, shrinking terminal windows, and cancellation must not produce false check failures, hang on prompts, or leave files behind. Test in Tasks 7, 15–17.

## Execution context and scope

The repository currently contains `go.mod`, `README.md`, `AGENTS.md`, the approved
spec, and a printing stub at `cmd/cli/main.go`. There are no existing product
packages or tests. The current local toolchain is Go 1.26.7. No product dependency
has been installed or source file changed while writing this plan.

This is one coordinated command, with three implementation tracks: the shared
engine (Tasks 1–7), supported application templates (Tasks 8–13), and style,
tooling, wizard, and release validation (Tasks 14–17). Template tasks have distinct
reviewable outputs, but depend on the same model and engine; separate product
specs would duplicate those contracts. Finish all tracks before calling v1 done.

At execution time, use `superpowers:using-git-worktrees` to establish an isolated
workspace, then the execution skill selected by the user. Each task follows a
failing-test → implementation → passing-test → commit cycle. Commit only the
listed task paths. Do not treat example snippets as exemptions from the spec's
documentation rules; exported production declarations get short doc comments.

## Dependencies and source checks

The following direct versions were checked against primary sources or the local
module cache on 2026-09-24. Pin them when their owning task needs them, commit
`go.sum`, and run with `GOTOOLCHAIN=local` to catch accidental version-floor changes.
Transitive module selection is recorded by the Go toolchain during implementation.

| Use | Module | Pin |
| --- | --- | --- |
| Strict YAML with comment-preserving nodes | `go.yaml.in/yaml/v3` | `v3.0.5` |
| Module parsing | `golang.org/x/mod` | `v0.41.0` |
| Rubric and generated Cobra CLI | `github.com/spf13/cobra` | `v1.10.2` |
| Rubric and generated TUI | `charm.land/bubbletea/v2` | `v2.0.9` |
| Terminal detection | `github.com/charmbracelet/x/term` | `v0.2.2` |
| Terminal smoke tests | `github.com/creack/pty` | `v1.1.24` |
| Generated Chi | `github.com/go-chi/chi/v5` | `v5.3.2` |
| Generated Viper | `github.com/spf13/viper` | `v1.21.0` |
| Generated SQLite | `modernc.org/sqlite` | `v1.59.0` |
| Generated PostgreSQL, including its `stdlib` adapter | `github.com/jackc/pgx/v5` | `v5.11.0` |
| Generated SQL unit tests | `github.com/DATA-DOG/go-sqlmock` | `v1.5.2` |
| sqlc launcher | `github.com/sqlc-dev/sqlc/cmd/sqlc` | `v1.31.1` |
| Lint launcher | `github.com/golangci/golangci-lint/v2/cmd/golangci-lint` | `v2.13.2` |

Use these immutable Actions refs in both Rubric CI and rendered workflows:

```text
actions/checkout@08c6903cd8c0fde910a37f88322edcfb5dd907a8
actions/setup-go@44694675825211faa026b3c33043df3e48a5fa00
```

Sources: [Bubble Tea v2 module](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/go.mod)
and [API examples](https://github.com/charmbracelet/bubbletea/tree/v2.0.9),
[YAML API](https://pkg.go.dev/go.yaml.in/yaml/v3@v3.0.5),
[Cobra](https://github.com/spf13/cobra/tree/v1.10.2),
[Chi](https://github.com/go-chi/chi/tree/v5.3.2),
[Viper](https://github.com/spf13/viper/tree/v1.21.0),
[SQLite](https://pkg.go.dev/modernc.org/sqlite@v1.59.0),
[pgx](https://github.com/jackc/pgx/tree/v5.11.0),
[sqlc](https://github.com/sqlc-dev/sqlc/tree/v1.31.1),
[golangci-lint](https://github.com/golangci/golangci-lint/tree/v2.13.2),
[checkout commit](https://github.com/actions/checkout/commit/08c6903cd8c0fde910a37f88322edcfb5dd907a8),
[setup-go commit](https://github.com/actions/setup-go/commit/44694675825211faa026b3c33043df3e48a5fa00).

## File structure and ownership

| Location | Responsibility |
| --- | --- |
| `cmd/rubric/main.go` | Thin process entry; replaces `cmd/cli/main.go`. |
| `internal/config/` | Versioned schema, presence-aware inputs, defaults, validation, YAML updates. |
| `internal/detect/` | Read-only repository facts with evidence. |
| `internal/catalog/` | Supported combinations, pinned dependencies, template revision. |
| `internal/render/` | Pure file composition, commands, instructions, embedded templates. |
| `internal/plan/` | Snapshots, hashes, managed sections, actions, conflict decisions. |
| `internal/write/` | Contained writes, concurrency checks, rollback, cancellation. |
| `internal/initialize/` | Prepare/apply service shared by both interfaces. |
| `internal/cli/` | Cobra parsing, exit classification, text/JSON reporting. |
| `internal/wizard/` | Bubble Tea pages, state transitions, preview and apply messages. |
| `internal/style/` | Standard-library-only comment analyzer and exportable source assets. |
| `internal/testproject/` | Shared test helpers for generated projects and subprocesses. |
| `internal/acceptance/` | Full generation matrix, PostgreSQL integration, terminal smoke tests. |
| `.github/workflows/ci.yml` | Tests, generated-project matrix shards, coverage, and style checks. |
| `docs/cli-init.md` | User-facing usage, output, setup, conflicts, and test commands. |

Each task below names exact files. All paths are relative to the implementation
worktree. Tests default to the standard library; sqlmock is used only where its
database boundary assertions give meaningful behavior coverage.

## Shared contracts

Task 1 defines these schema types in `internal/config/types.go`. Every field has
matching lower-case `yaml` and `json` tags (for example `yaml:"schema" json:"schema"`).
The table below fixes their names and types; no task invents a parallel model.

| Type | Fields |
| --- | --- |
| `Config` | `Schema int`, `Project Project`, `Features Features`, `Tooling Tooling`, `EntryPoints []EntryPoint`, `Commands []Command`, `Evidence []Evidence`, `Generator Generator`, `Style string` |
| `Project` | `Module string`, `Name string`, `Description string`, `Go string`, `Starter string` |
| `Features` | `HTTP string`, `Database string`, `Access string`, `CLI string`, `TUI string`, `Config string` |
| `Tooling` | `Skills bool`, `Lint bool`, `Makefile bool`, `Actions bool` |
| `EntryPoint` | `Name string`, `Dir string` |
| `Command` | `Name string`, `Dir string`, `Argv []string`, `Env []string` (required environment names, never values) |
| `Evidence` | `Field string`, `Value string`, `Source string` |
| `Generator` | `Version string`, `Template int`, `Go string`, `Dependencies map[string]string`, `Tools map[string]string` |
| `Patch` | `map[string]any`, keyed by dot-separated schema paths; lists replace atomically |
| `Document` | `Node *yaml.Node`, `Values Patch` |

Persist fields as `entry_points`, `module`, `starter`, `http`, `database`, `access`,
`cli`, `tui`, `config`, and the other lower-case field names. Enum values are:
`module|runnable` for starter; `none|nethttp|chi` for HTTP;
`none|sqlite|postgres` for database; `none|sql|sqlc` for access;
`none|flag|cobra` for CLI; `none|bubbletea` for TUI; `stdlib|viper` for config.
Schema is `1`, style is `rubric-go-v1`, template revision is `1`, generator Go
baseline is `1.26.7`, and development generator version is `dev` (release builds
inject their version). Keep the generator/tool Go baseline distinct from an
existing application's preserved Go directive.

Modes `auto|new|existing`, destination paths, output format, and dry-run state are
invocation data, not saved machine-local fields. Existing-project feature values
may identify other libraries; validate generation enums only for new projects.

Task 3 defines `render.File` with `Path string`, `Data []byte`, `Mode fs.FileMode`,
and `Kind string`. Kinds are `scaffold`, `managed`, `guidance`, and `config`.
Task 5 defines `plan.Plan` with `Mode string`, `Config config.Config`,
`Actions []Action`, `Obsolete []string`; `Action` contains `File render.File`,
`Before Snapshot`, `State string`, and `Reason string`. States are `create`,
`update`, `unchanged`, `conflict`, and `skip`. `Snapshot` contains `Exists bool`,
`Data []byte`, `Mode fs.FileMode`, and `Hash string`; hashes are SHA-256 hex.

## Task 1: Presence-aware configuration and editable YAML

**Files:** Create `internal/config/types.go`, `internal/config/defaults.go`, `internal/config/decode.go`,
`internal/config/resolve.go`, `internal/config/validate.go`, `internal/config/encode.go`, and `internal/config/config_test.go`; modify `go.mod`
and create `go.sum` as dependencies are introduced.

**Interfaces:** Produces `Defaults() Config`, `Decode([]byte) (Document, error)`,
`Resolve(Config, ...Patch) (Config, error)`, `Validate(Config, string) error`, and
`Encode(Document, Config) ([]byte, error)`. The string in `Validate` is the resolved
mode. Decode requires one YAML document, preserves the syntax tree, rejects
duplicate/unknown keys and cycles, and retains the presence of explicit values.

- [ ] **Step 1: Add a failing override test and a commented-YAML round-trip test.**

```go
func TestExplicitDisabledValuesWin(t *testing.T) {
    base := Defaults()
    base.Tooling.Lint = true
    base.Features.HTTP = "chi"
    base.Commands = []Command{{Name: "run", Dir: ".", Argv: []string{"go", "run", "."}}}
    got, err := Resolve(base, Patch{
        "tooling.lint": false, "features.http": "none", "commands": []Command{},
    })
    if err != nil { t.Fatal(err) }
    if got.Tooling.Lint || got.Features.HTTP != "none" || len(got.Commands) != 0 {
        t.Fatalf("explicit disabled settings lost: %+v", got)
    }
}
```

Add table cases for wrong types, duplicate keys, a second YAML document, aliases
with cycles, unknown keys, schema `2`, invalid enum combinations, malformed module
paths, an empty module on new mode, Unicode descriptions, and Go-version mismatch.
Check that plain local module names accepted by Go are not rejected just because
they lack a public hostname, while invalid import characters still fail.
The round-trip test must compare preserved head/line/foot comments after changing
`features.http`, and assert no absolute destination path is serialized.

- [ ] **Step 2: Run `go test ./internal/config -run 'TestExplicit|TestYAML|TestValidate' -v`; expect missing production APIs.**
- [ ] **Step 3: Implement field-presence merging and typed validation.**

```go
func applyBool(dst *bool, values Patch, key string) error {
    value, ok := values[key]
    if !ok { return nil }
    enabled, ok := value.(bool)
    if !ok { return fmt.Errorf("%s: expected boolean", key) }
    *dst = enabled
    return nil
}
```

Use the same presence check for strings and complete-list replacement. Walk the
YAML node against the schema, reject unknown mappings before conversion, and
patch changed node values instead of remarshal-replacing the original document.
Defaults: schema `1`, Go `1.26.7`, starter `module`, capabilities `none`, config
`stdlib`, all tooling false. Enabling a database without an access choice defaults
to `sql`; explicit `access: none` with a database is invalid. Without a database,
access must be `none`. Disabling a database clears inherited access settings;
an explicit incompatible access choice in the same or a higher-precedence layer
is an error. Test both cases. A named executable normalizes starter to `module`;
without named executables, either starter is allowed, including projects with
only database or Viper support.
Validate new module paths with `module.CheckImportPath`; derive display name from
the last module segment when absent. Validate entry-point and command directories
as portable relative paths without `..`, absolute paths, or backslashes. Reject
NUL/newline in path-like values; descriptions are text, never command fragments.

Install only this task's dependencies:

```sh
go get go.yaml.in/yaml/v3@v3.0.5 golang.org/x/mod@v0.41.0
```

- [ ] **Step 4: Run `go test ./internal/config -v`; all merge, schema, and round-trip tests must pass.**
- [ ] **Step 5: Commit with `git add go.mod go.sum internal/config` and `git commit -m "feat: add rubric configuration model"`.**

## Task 2: Static detection for existing modules

**Files:** Create `internal/detect/detect.go`, `internal/detect/imports.go`, `internal/detect/entrypoints.go`,
and `internal/detect/detect_test.go`. Build fixtures inside each test's temporary directory.

**Interfaces:** Consumes `config.Config`, `config.Evidence`, and `config.EntryPoint`.
Produces `Inspect(string) (Facts, error)`, where `Facts` has `Module string`,
`Go string`, `Empty bool`, `Workspace bool`, `NestedModules []string`,
`EntryPoints []config.EntryPoint`, and `Evidence []config.Evidence`.

- [ ] **Step 1: Add a fixture with an unused Chi dependency and a real stdlib server.**

```go
func TestDependencyIsNotAnActiveComponent(t *testing.T) {
    root := t.TempDir()
    mod := "module example.com/app\n\ngo 1.26.7\n\nrequire github.com/go-chi/chi/v5 v5.3.2\n"
    if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(mod), 0o644); err != nil { t.Fatal(err) }
    facts, err := Inspect(root)
    if err != nil { t.Fatal(err) }
    for _, item := range facts.Evidence {
        if item.Field == "features.http" && item.Value == "chi" {
            t.Fatal("unused dependency classified as active HTTP component")
        }
    }
}
```

Also test unknown libraries, `package main` without a main function, nested
modules, `go.work` without a selected module, an explicit module beneath a
workspace, malformed source, ignored `.git`/`vendor`/`.rubric` paths, symlinks,
and a deleted entry point on rerun. Test an HTTP client-only source file and a
router imported only by tests; neither should prove the project runs a server.
Do not run `go list` for discovery.

- [ ] **Step 2: Run `go test ./internal/detect -v`; expect missing detector APIs.**
- [ ] **Step 3: Parse module and Go source without executing project code.**

```go
func importsIn(filename string, src []byte) ([]string, error) {
    f, err := parser.ParseFile(token.NewFileSet(), filename, src, parser.ImportsOnly)
    if err != nil { return nil, err }
    paths := make([]string, 0, len(f.Imports))
    for _, item := range f.Imports {
        value, err := strconv.Unquote(item.Path.Value)
        if err != nil { return nil, err }
        paths = append(paths, value)
    }
    slices.Sort(paths)
    return slices.Compact(paths), nil
}
```

Use `modfile.Parse` for module metadata and AST declarations for executable
entry points. Direct production imports identify libraries, while server/router
construction or serving calls supply evidence for an HTTP-server capability.
Label direct source imports with their source file; record a
`go.mod` dependency as dependency evidence only. Deduplicate/sort results.
Stop at nested `go.mod` boundaries. A top-level module can be explicitly selected
even if descendants contain independent modules; never scan those descendants
as part of the selected project. Distinguish absent, empty, ambiguous nonempty,
and valid existing-module targets in `initialize.Prepare` (Task 7).

- [ ] **Step 4: Run `go test ./internal/detect ./internal/config`; fixtures must remain byte-identical after inspection.**
- [ ] **Step 5: Commit with `git add internal/detect` and `git commit -m "feat: detect existing Go project facts"`.**

## Task 3: Pinned catalog, minimal rendering, and generated-project test helpers

**Files:** Create `internal/catalog/catalog.go`, `internal/catalog/catalog_test.go`,
`internal/render/files.go`, `internal/render/render.go`, `internal/render/render_test.go`,
`internal/render/templates/base/go.mod.tmpl`, `internal/render/templates/base/main.go.tmpl`,
`internal/testproject/project.go`, and `internal/testproject/project_test.go`.

**Interfaces:** Produces `catalog.Dependencies(config.Features) (map[string]string, error)`,
`catalog.Cases() []config.Features`, `render.Files(config.Config, string) ([]File, error)`.
The string is mode; existing mode emits no application source or `go.mod`.
Test helpers are `testproject.Config() config.Config`,
`File(t *testing.T, files []render.File, path string) []byte`,
`Write(t *testing.T, files []render.File) string`, and
`Go(t *testing.T, dir string, args ...string) string` (combined output; fatal on error).

- [ ] **Step 1: Test the two empty starters and literal module rendering.**

```go
func TestMinimalStarters(t *testing.T) {
    for _, starter := range []string{"module", "runnable"} {
        t.Run(starter, func(t *testing.T) {
            c := testproject.Config()
            c.Project.Starter = starter
            files, err := render.Files(c, "new")
            if err != nil { t.Fatal(err) }
            root := testproject.Write(t, files)
            if starter == "runnable" { testproject.Go(t, root, "build", "./...") }
            if _, err := os.Stat(filepath.Join(root, "main_test.go")); !errors.Is(err, fs.ErrNotExist) {
                t.Fatalf("unnecessary main test: %v", err)
            }
        })
    }
}
```

Check deterministic byte order, no duplicate output paths, correct source file
permissions, and no application source on existing mode. Reject invalid module
paths before using them in Go imports; quote data using `strconv.Quote`, not raw
concatenation. Test descriptions containing `$()`, backticks, quotes, and Unicode
as literal documentation data. Test all 180 unique catalog combinations.

- [ ] **Step 2: Run `go test ./internal/catalog ./internal/render ./internal/testproject`; expect missing APIs.**
- [ ] **Step 3: Embed templates and format generated Go before returning bytes.**

```go
func renderSource(tmpl *template.Template, value any) ([]byte, error) {
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, value); err != nil { return nil, err }
    return format.Source(buf.Bytes())
}
```

Base `main.go.tmpl` is `package main` with `func main() {}`. Add no fake test.
`catalog.Cases` enumerates HTTP 3 × database/access 5 × CLI 3 × TUI 2 × config 2.
Dependencies come from the fixed table above; their map is rendered in sorted
module-path order. `testproject.Go` uses `exec.CommandContext(t.Context(), "go", args...)`,
sets `Dir`, `GOWORK=off`, and `GOTOOLCHAIN=local`, retains useful failure output,
and never builds shell command strings. Add helper tests for literal argument
passing and temporary-directory cleanup. `testproject.Config` returns defaults
with module `example.com/demo` and display name `demo`.

- [ ] **Step 4: Run those package tests; build the runnable starter and inspect module-only output for absence of application files.**
- [ ] **Step 5: Commit with `git add internal/catalog internal/render internal/testproject` and `git commit -m "feat: render minimal Go starters"`.**

## Task 4: Command inventory and accurate project guidance

**Files:** Create `internal/render/commands.go`, `internal/render/guidance.go`, `internal/render/guidance_test.go`,
`internal/render/templates/base/README.md.tmpl`, `internal/render/templates/base/AGENTS.md.tmpl`, and
`internal/render/templates/base/style.md.tmpl`; modify `internal/render/render.go`.

**Interfaces:** Produces `Commands(config.Config, string) []config.Command` and
`Instructions(config.Config) ([]byte, error)`. `render.Files` includes canonical
`rubric.yaml`, README for new projects, the AGENTS managed-section content, and
`.rubric/style.md`. The current YAML document is applied by the service in Task 7.

- [ ] **Step 1: Test that a module-only project does not advertise imaginary commands.**

```go
func TestModuleOnlyGuidanceHasNoRunCommand(t *testing.T) {
    c := testproject.Config()
    c.Commands = render.Commands(c, "new")
    text, err := render.Instructions(c)
    if err != nil { t.Fatal(err) }
    if strings.Contains(string(text), "go run .") || strings.Contains(string(text), "cmd/server") {
        t.Fatalf("guidance invents an executable:\n%s", text)
    }
    if !strings.Contains(string(text), "main.go") || !strings.Contains(string(text), "cmd/") {
        t.Fatal("generated testing exemptions missing")
    }
}
```

Add cases for each executable path, existing custom commands, missing commands,
unselected tooling, all three style links, the two-line/150-character rule, and
the absence of unfinished `rubric validate` or performance instructions.

- [ ] **Step 2: Run `go test ./internal/render -run 'TestGuidance|TestModuleOnly' -v`; new assertions must fail first.**
- [ ] **Step 3: Derive commands once and render every artifact from that inventory.**

```go
func runCommand(name, dir string) config.Command {
    return config.Command{Name: name, Dir: ".", Argv: []string{"go", "run", dir}}
}
```

For new named executables, inventory `run-server`, `run-cli`, and `run-tui` only
for selected components; a root starter gets `run`. Base checks are `go build
./...` and `go test ./...` when packages exist. Conditional tooling wrappers in
Task 15 handle the empty-module case without claiming tests ran. Existing
commands are explicit inputs or verified entry-point commands, never inferred
from an unexecuted README shell snippet. AGENTS markers are exactly
`<!-- rubric:begin -->` and `<!-- rubric:end -->`; return only the managed section
from `Instructions`, leaving placement and preservation to Task 5.

- [ ] **Step 4: Run `go test ./internal/render`; assert instructions agree with rendered paths and selected features.**
- [ ] **Step 5: Commit with `git add internal/render` and `git commit -m "feat: generate project-aware agent guidance"`.**

## Task 5: Preview plans, ownership, and conflicts

**Files:** Create `internal/plan/types.go`, `internal/plan/manifest.go`, `internal/plan/snapshot.go`,
`internal/plan/guidance.go`, `internal/plan/prepare.go`, `internal/plan/decisions.go`, and `internal/plan/plan_test.go`.

**Interfaces:** Produces `Prepare(string, string, config.Config, []render.File) (Plan, error)`
(target, mode, config, files), `Decide(Plan, map[string]string) (Plan, error)`,
and `Conflicts(Plan) []Action`. Decision values are `replace` or `skip`; mandatory
config/guidance cannot be skipped. `Manifest` contains `Version int` and
`Files map[string]Stamp`; `Stamp` has `Hash string` and `Kind string`.

- [ ] **Step 1: Test unmanaged file preservation and marker parsing.**

```go
func TestExistingUserFileIsAConflict(t *testing.T) {
    root := t.TempDir()
    path := filepath.Join(root, "Makefile")
    if err := os.WriteFile(path, []byte("user-target:\n\ttrue\n"), 0o644); err != nil { t.Fatal(err) }
    files := []render.File{{Path: "Makefile", Data: []byte("new-target:\n"), Mode: 0o644, Kind: "managed"}}
    p, err := Prepare(root, "existing", testproject.Config(), files)
    if err != nil { t.Fatal(err) }
    if len(Conflicts(p)) != 1 { t.Fatalf("want one conflict: %+v", p.Actions) }
    data, err := os.ReadFile(path)
    if err != nil { t.Fatal(err) }
    if string(data) != "user-target:\n\ttrue\n" { t.Fatal("preview wrote a file") }
}
```

Test identical unmanaged files, malformed/duplicated AGENTS markers, user content
outside markers, edited managed sections, absent manifests, edited manifests,
normal `rubric.yaml` edits, obsolete paths, concurrent changes, and unchanged
reruns. Reject corrupt manifests rather than adopting their contents silently.

- [ ] **Step 2: Run `go test ./internal/plan -v`; expect missing planning APIs.**
- [ ] **Step 3: Hash content and classify changes without writing.**

```go
func digest(data []byte) string {
    sum := sha256.Sum256(data)
    return hex.EncodeToString(sum[:])
}
```

Snapshot every affected target, including manifest and input configuration. Hash
only the managed AGENTS section for ownership but retain the whole file's preview
snapshot for concurrency checks. `rubric.yaml` uses current-input comparison,
not its historical manifest hash. Never hash the manifest into itself.
Do not own identical preexisting user files merely because they match a template.
Decisions apply only to the reviewed bytes; if a skip disables an optional
artifact, the service revises tooling selections and recomputes commands and
guidance before producing a new preview. Preserve obsolete files and list them.

- [ ] **Step 4: Run `go test ./internal/plan`; compare directory snapshots before/after every preview-only test.**
- [ ] **Step 5: Commit with `git add internal/plan` and `git commit -m "feat: plan file ownership and conflicts"`.**

## Task 6: Safe application and rollback

**Files:** Create `internal/write/apply.go`, `internal/write/root.go`, `internal/write/journal.go`, `internal/write/rollback.go`,
`internal/write/write_test.go`, and `internal/write/rollback_test.go`.

**Interfaces:** Produces `Apply(context.Context, string, plan.Plan) (Result, error)`.
`Result` has `Applied []string`, `Restored []string`, and `Unrecovered []string`.
Expose typed errors `ConflictError` (`Paths []string`) and `ApplyError`
(`Cause error`, `Result Result`) with `Error`/`Unwrap` methods.
Keep injectable filesystem operations private to package tests, via `applyWithOps`;
its arguments are the public Apply arguments plus an `operations` struct holding
read, stat, write, rename, remove, mkdir, and chmod functions.

- [ ] **Step 1: Pin preflight conflict and cancellation behavior.**

```go
func TestCancelledApplyCreatesNothing(t *testing.T) {
    root := filepath.Join(t.TempDir(), "new project")
    files := []render.File{{Path: "go.mod", Data: []byte("module example.com/app\n"), Mode: 0o644, Kind: "scaffold"}}
    p, err := plan.Prepare(root, "new", testproject.Config(), files)
    if err != nil { t.Fatal(err) }
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    if _, err := Apply(ctx, root, p); !errors.Is(err, context.Canceled) { t.Fatalf("got %v", err) }
    if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) { t.Fatalf("target was created: %v", err) }
}
```

Inject failure at every replacement index. Assert earlier files restore, new
files/directories are removed only if owned by this invocation, unrelated files
survive, and rollback refuses to overwrite a concurrent user edit. Test symlink
parents and targets, `../` paths, an absolute file path, literal special characters
in destination names, permissions, a stale manifest, and cancellation mid-apply.

- [ ] **Step 2: Run `go test ./internal/write -v`; expect missing writer APIs.**
- [ ] **Step 3: Stage, recheck, replace, and journal each file.**

```go
func unchanged(before plan.Snapshot, current []byte, exists bool) bool {
    return before.Exists == exists && (!exists || bytes.Equal(before.Data, current))
}
```

Use `os.Root`-relative operations after validating the selected destination and
rejecting symlink write paths. For a missing destination, first root operations
at its nearest existing ancestor and create only planned directories. Preflight
all snapshots before mutation, then recheck immediately before each replacement.
Write temporary siblings with exclusive creation, close and set modes before
rename, and update the manifest last. Journal old content/mode and the digest of
this invocation's replacement. Roll back in reverse order only when the current
content still matches that replacement. Preserve ambiguous concurrent changes
and return their paths as unrecovered. Clean staged files on all exit paths.

- [ ] **Step 4: Run `go test -race ./internal/write ./internal/plan`; all failure-injection cases must pass.**
- [ ] **Step 5: Commit with `git add internal/write` and `git commit -m "feat: apply initialization plans with recovery"`.**

## Task 7: Shared initialization service and unattended CLI

**Files:** Move `cmd/cli/main.go` to `cmd/rubric/main.go`; create
`internal/initialize/request.go`, `internal/initialize/prepare.go`, `internal/initialize/apply.go`, `internal/initialize/initialize_test.go`,
`internal/cli/command.go`, `internal/cli/flags.go`, `internal/cli/report.go`, `internal/cli/exit.go`, and `internal/cli/cli_test.go`;
modify `go.mod` and `go.sum` for Cobra.

**Interfaces:** `initialize.Request` has `Target string`, `Mode string`,
`Input []byte`, `Overrides config.Patch`, and `Decisions map[string]string`.
Produces `initialize.Prepare(context.Context, Request) (plan.Plan, error)` and
`initialize.Apply(context.Context, Request, plan.Plan) (write.Result, error)`.
CLI produces `Run(context.Context, []string, Streams) int`; `Streams` contains
`In io.Reader`, `Out io.Writer`, `Err io.Writer`, `Terminal bool`.
`initialize.InputError` has `Cause error`; `initialize.ConflictError` has
`Plan plan.Plan`. Both implement `Error() string`; InputError also implements
`Unwrap() error`. Those errors and `write.ConflictError` map to status 2; context cancellation maps
to 130; other errors map to 1. Success/dry-run without conflicts maps to 0.

- [ ] **Step 1: Test a fully unattended dry run into a nonexistent directory.**

```go
func TestJSONDryRunDoesNotWrite(t *testing.T) {
    root := filepath.Join(t.TempDir(), "new project")
    var out, stderr bytes.Buffer
    args := []string{"init", root, "--module", "example.com/demo", "--format", "json", "--dry-run"}
    code := Run(t.Context(), args, Streams{In: strings.NewReader(""), Out: &out, Err: &stderr})
    if code != 0 { t.Fatalf("exit %d: %s", code, stderr.String()) }
    if !json.Valid(out.Bytes()) { t.Fatalf("invalid JSON: %s", out.String()) }
    if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) { t.Fatalf("dry run wrote files: %v", err) }
}
```

Add process-level cases for help, unknown command/flag, config/flag precedence,
missing module, unknown schema, explicit `--lint=false`, dry-run conflicts,
JSON stderr separation, operational failure, SIGINT, existing project with
unknown libraries, nonempty non-module targets, and rerun preservation.

- [ ] **Step 2: Run `go test ./internal/initialize ./internal/cli`; expect missing service/CLI APIs.**
- [ ] **Step 3: Implement the service pipeline and thin process wrapper.**

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    code := cli.Run(ctx, os.Args[1:], cli.Streams{
        In: os.Stdin, Out: os.Stdout, Err: os.Stderr,
        Terminal: term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()),
    })
    stop()
    os.Exit(code)
}
```

Install Cobra and terminal detection at the pins above. Resolve mode from actual
files, not saved "new project" intent; reject `--mode new` for an existing module.
Merge defaults, fresh detection, saved target configuration, explicit input, then
changed flags. Retain detection evidence and surface stale saved facts: unattended
stale entry-point commands fail with a correction message rather than advertise
missing files. Reconcile generated ownership with deleted components on rerun.
Fill derived command and entry-point fields only when absent from all explicit
user layers; preserve explicitly empty lists instead of repopulating them. Keep
that presence information until normalization finishes.
Call `config.Encode` using the current target YAML node and replace only the
planned `rubric.yaml` bytes before `plan.Prepare` snapshots are made.

Flags are `--mode`, `--module`, `--name`, `--description`, `--starter`, `--http`,
`--database`, `--access`, `--cli`, `--tui`, `--app-config`, `--skills`, `--lint`,
`--makefile`, `--actions`, `--config`, `--non-interactive`, `--dry-run`, and
`--format`. Add repeatable JSON-valued `--entry-point` and `--command` flags to
represent structured existing-project data, plus `--clear-entry-points` and
`--clear-commands` for explicit empty lists. Never parse command data with a shell.
Only Cobra flags with `Changed` set become overrides.

Until Task 16 attaches the wizard, keep terminal execution routed through the
same complete-input service rather than claiming the interactive product is done.
JSON contains mode/config/actions/conflicts/diagnostics/next commands, exactly
once; do not emit raw rendered file contents or environment values by default.

- [ ] **Step 4: Run `go test ./internal/config ./internal/detect ./internal/render ./internal/plan ./internal/write ./internal/initialize ./internal/cli` and `go build ./cmd/rubric`.**
- [ ] **Step 5: Commit with `git add cmd internal/initialize internal/cli go.mod go.sum` and `git commit -m "feat: add unattended rubric init"`.**

## Task 8: HTTP templates with handler and lifecycle tests

**Files:** Create `internal/render/http.go`, `internal/render/http_test.go`,
`internal/render/templates/http/nethttp.go.tmpl`, `internal/render/templates/http/chi.go.tmpl`,
`internal/render/templates/http/server.go.tmpl`, `internal/render/templates/http/server_test.go.tmpl`,
`internal/render/templates/http/main.go.tmpl`; modify `internal/render/render.go` and `internal/render/commands.go`.

**Interfaces:** Generated `internal/httpserver` exports
`New(func(context.Context) error) http.Handler` and
`Serve(context.Context, net.Listener, func(context.Context) error) error`.
The optional function is a readiness check; nil means no external dependency.
Output files are `internal/httpserver/routes.go`, `server.go`, `server_test.go`,
and `cmd/server/main.go`. Both router choices have the same API and tests.

- [ ] **Step 1: Add this generated test and a render test that runs it for both routers.**

```go
func TestHealth(t *testing.T) {
    handler := New(nil)
    response := httptest.NewRecorder()
    handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
    if response.Code != http.StatusOK || response.Body.String() != "ok\n" {
        t.Fatalf("health = %d %q", response.Code, response.Body.String())
    }
}
```

Add readiness success/failure/cancellation, unknown route, wrong method, listen
failure through the supplied listener, and graceful shutdown tests. The render
test uses `testproject.Write`, `go mod tidy`, `go build ./...`, and `go test ./...`.
Assert the generated tests exist even with all tooling disabled.

- [ ] **Step 2: Run `go test ./internal/render -run TestGeneratedHTTP -v`; expect absent templates/output.**
- [ ] **Step 3: Implement shared handlers and router-specific registration.**

```go
func health(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    _, err := io.WriteString(w, "ok\n")
    if err != nil { return }
}
```

Use method-specific registration on both routers. Readiness calls the injected
function with the request context and returns 503 with a fixed body on error.
`Serve` shuts down with a bounded context on cancellation, waits for completion,
and treats normal `http.ErrServerClosed` as success. Tests exercise listener
errors and shutdown without leaking goroutines. Keep main limited to signal
context, listener construction, startup, and exit; Task 11 centralizes settings.

- [ ] **Step 4: Run `go test ./internal/render -run TestGeneratedHTTP -v`; both generated projects must build and pass their shipped tests.**
- [ ] **Step 5: Commit with `git add internal/render` and `git commit -m "feat: generate tested HTTP services"`.**

## Task 9: Standard-flag and Cobra CLI templates

**Files:** Create `internal/render/appcli.go`, `internal/render/appcli_test.go`,
`internal/render/templates/cli/flag.go.tmpl`, `internal/render/templates/cli/cobra.go.tmpl`,
`internal/render/templates/cli/command_test.go.tmpl`, and `internal/render/templates/cli/main.go.tmpl`;
modify `internal/render/render.go` and `internal/render/commands.go`.

**Interfaces:** Generated `internal/cli.Run(context.Context, []string, io.Writer,
func(context.Context) error) error`; output `internal/cli/command.go`,
`command_test.go`, and `cmd/cli/main.go`. Both choices implement a `status`
command and expose help without requiring a database.

- [ ] **Step 1: Ship the same command behavior test with both implementations.**

```go
func TestStatus(t *testing.T) {
    var out bytes.Buffer
    calls := 0
    err := Run(t.Context(), []string{"status"}, &out, func(context.Context) error {
        calls++
        return nil
    })
    if err != nil { t.Fatal(err) }
    if out.String() != "ok\n" || calls != 1 {
        t.Fatalf("output=%q checks=%d", out.String(), calls)
    }
}
```

Add invalid flags, unknown commands, help, canceled contexts, readiness failure,
and output-writer failure. Assert two concurrent Run calls do not share state.

- [ ] **Step 2: Run `go test ./internal/render -run TestGeneratedCLI -v`; expect missing generated CLI files.**
- [ ] **Step 3: Use fresh flag sets or Cobra commands on every Run call.**

```go
func status(ctx context.Context, out io.Writer, check func(context.Context) error) error {
    if err := ctx.Err(); err != nil { return err }
    if check != nil {
        if err := check(ctx); err != nil { return err }
    }
    _, err := io.WriteString(out, "ok\n")
    return err
}
```

Use `flag.ContinueOnError` for the stdlib variant. Cobra returns errors to main
instead of terminating inside the package; set its context/output explicitly.
Main forwards arguments and handles the final exit. Shared application packages
must not import either generated `main` package.

- [ ] **Step 4: Run `go test ./internal/render -run TestGeneratedCLI -v`; compile and run each generated suite, including HTTP+CLI compositions.**
- [ ] **Step 5: Commit with `git add internal/render` and `git commit -m "feat: generate tested CLI applications"`.**

## Task 10: Generated Bubble Tea application

**Files:** Create `internal/render/apptui.go`, `internal/render/apptui_test.go`,
`internal/render/templates/tui/model.go.tmpl`, `internal/render/templates/tui/model_test.go.tmpl`, and
`internal/render/templates/tui/main.go.tmpl`; modify `internal/render/render.go` and `internal/render/commands.go`.

**Interfaces:** Generated `internal/tui.New(context.Context, func(context.Context) error) Model`;
`Model` implements `Init() tea.Cmd`, `Update(tea.Msg) (tea.Model, tea.Cmd)`, and
`View() tea.View`. Output `internal/tui/model.go`, `model_test.go`, and
`cmd/tui/main.go`. Use `charm.land/bubbletea/v2`, not v1 import paths or view types.

- [ ] **Step 1: Ship a terminal-independent quit test.**

```go
func TestQuit(t *testing.T) {
    model := New(t.Context(), nil)
    _, command := model.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
    if command == nil { t.Fatal("quit key produced no command") }
    if _, ok := command().(tea.QuitMsg); !ok { t.Fatal("expected quit message") }
}
```

Test initial content, window resize including zero dimensions, readiness success
and failure messages, unrelated messages, and context cancellation. Run returned
commands directly in tests instead of opening a real terminal.

- [ ] **Step 2: Run `go test ./internal/render -run TestGeneratedTUI -v`; expect missing generated model.**
- [ ] **Step 3: Keep I/O in commands and state changes in Update.**

```go
func (m Model) View() tea.View {
    return tea.NewView(m.status + "\nPress q to quit.\n")
}
```

The model holds `ctx context.Context`, `check func(context.Context) error`,
`status string`, `width int`, and `height int`. `Init` returns a command producing
an unexported `checkedMsg{err error}` when a check is supplied. Main uses
`tea.NewProgram(model, tea.WithContext(ctx))` and handles its result. Generated
TUI tests run without a database service and remain present without CI/Makefile.

- [ ] **Step 4: Run `go test ./internal/render -run TestGeneratedTUI -v`; also build a combined HTTP+CLI+TUI project.**
- [ ] **Step 5: Commit with `git add internal/render` and `git commit -m "feat: generate tested Bubble Tea applications"`.**

## Task 11: Standard-library and Viper runtime configuration

**Files:** Create `internal/render/appconfig.go`, `internal/render/appconfig_test.go`,
`internal/render/templates/config/stdlib.go.tmpl`, `internal/render/templates/config/viper.go.tmpl`,
`internal/render/templates/config/config_test.go.tmpl`, `internal/render/templates/config/app.yaml.tmpl`;
modify `internal/render/templates/http/main.go.tmpl`,
`internal/render/templates/cli/main.go.tmpl`, `internal/render/templates/tui/main.go.tmpl`,
`internal/render/templates/base/main.go.tmpl`, `internal/render/render.go`,
`internal/render/commands.go`, and `internal/render/guidance.go`.

**Interfaces:** Generated `internal/config.Settings` has `HTTPAddr string` and
`DatabaseURL string`. Generated `Load(string, func(string) (string, bool)) (Settings, error)`
accepts an optional config-file path and explicit environment lookup function.
Generate this package for named executables, database support, or explicit Viper;
do not add it to either otherwise-empty starter. Runtime config belongs to the
application; Rubric's `rubric.yaml` is not passed to Viper.

- [ ] **Step 1: Ship an environment-override test isolated from the user's environment.**

```go
func TestEnvironmentOverridesDefaults(t *testing.T) {
    lookup := func(key string) (string, bool) {
        if key == "APP_HTTP_ADDR" { return "127.0.0.1:9090", true }
        return "", false
    }
    got, err := Load("", lookup)
    if err != nil { t.Fatal(err) }
    if got.HTTPAddr != "127.0.0.1:9090" { t.Fatalf("address=%q", got.HTTPAddr) }
}
```

Viper cases also cover an explicit missing file, malformed YAML, type errors,
file-over-default and environment-over-file precedence, two isolated instances,
and Cobra being absent. Test database settings only in templates that select a
database; distinguish missing PostgreSQL configuration from invalid settings.
When the environment-override test is emitted for a PostgreSQL project, its
lookup also supplies `APP_DATABASE_URL` as
`postgres://postgres@127.0.0.1:5432/rubric_test?sslmode=disable`; the test validates
configuration without making a connection. Missing-URL behavior has a separate test.

- [ ] **Step 2: Run `go test ./internal/render -run TestGeneratedConfig -v`; new settings assertions must fail first.**
- [ ] **Step 3: Implement local configuration instances and shared validation.**

```go
v := viper.New()
v.SetDefault("http_addr", ":8080")
if filename != "" {
    v.SetConfigFile(filename)
    if err := v.ReadInConfig(); err != nil { return Settings{}, err }
}
if value, ok := lookup("APP_HTTP_ADDR"); ok { v.Set("http_addr", value) }
if value, ok := lookup("APP_DATABASE_URL"); ok { v.Set("database_url", value) }
```

Decode into typed settings and reject invalid addresses. Viper uses file →
explicit supplied environment → defaults; do not use its process-global instance
or unbounded automatic file discovery. Stdlib mode supports explicit environment
values and errors on a nonempty file path. Main supplies `APP_CONFIG_FILE` and
`os.LookupEnv`; the optional sample `app.yaml` has no credentials. Document only
settings relevant to selected components. HTTP consumes the configured address;
Task 12 wires the database URL into all selected application entry points.

- [ ] **Step 4: Run `go test ./internal/render -run 'TestGeneratedConfig|TestGeneratedHTTP|TestGeneratedCLI|TestGeneratedTUI' -v`.**
- [ ] **Step 5: Commit with `git add internal/render` and `git commit -m "feat: generate optional Viper configuration"`.**

## Task 12: SQLite and PostgreSQL with handwritten SQL

**Files:** Create `internal/render/database.go`, `internal/render/database_test.go`,
`internal/render/templates/store/store.go.tmpl`, `internal/render/templates/store/sql.go.tmpl`,
`internal/render/templates/store/store_test.go.tmpl`, `internal/render/templates/store/sqlite_test.go.tmpl`,
`internal/render/templates/store/postgres_integration_test.go.tmpl`,
`internal/render/templates/store/schema.sqlite.sql.tmpl`, `internal/render/templates/store/schema.postgres.sql.tmpl`;
modify `internal/render/render.go`, `internal/render/appconfig.go`,
`internal/render/guidance.go`, `internal/render/templates/http/main.go.tmpl`,
`internal/render/templates/cli/main.go.tmpl`, `internal/render/templates/tui/main.go.tmpl`,
`internal/render/templates/base/main.go.tmpl`, and both config implementation templates.

**Interfaces:** Generated `Store` exports `New(*sql.DB) (*Store, error)`,
`Open(string) (*Store, error)`, `Ping(context.Context) error`,
`Get(context.Context, string) (string, error)`,
`Put(context.Context, string, string) error`, and `Close() error`.
`Open` uses the selected driver name (`sqlite` or `pgx`); driver side-effect
imports are in executable mains and tests, as the Go review guide recommends.
Library-only guidance explains the consumer's driver registration requirement.
Update the root runnable main template as well when supporting libraries are
selected without named executable components.

- [ ] **Step 1: Ship a real temporary-SQLite behavior test.**

```go
func TestRoundTrip(t *testing.T) {
    db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
    if err != nil { t.Fatal(err) }
    t.Cleanup(func() { if err := db.Close(); err != nil { t.Error(err) } })
    if _, err := db.Exec("CREATE TABLE entries (key TEXT PRIMARY KEY, value TEXT NOT NULL)"); err != nil { t.Fatal(err) }
    store, err := New(db)
    if err != nil { t.Fatal(err) }
    if err := store.Put(t.Context(), "name", "rubric"); err != nil { t.Fatal(err) }
    got, err := store.Get(t.Context(), "name")
    if err != nil || got != "rubric" { t.Fatalf("Get = %q, %v", got, err) }
}
```

Ship PostgreSQL unit tests with sqlmock for parameter binding, row decoding,
not-found, execution/scan errors, ping failure, cancellation, and close errors.
PostgreSQL integration tests use `//go:build integration` and `TEST_DATABASE_URL`;
when explicitly enabled, a missing URL fails instead of silently skipping.
Use a transaction and per-test schema to keep real database tests isolated.

- [ ] **Step 2: Run `go test ./internal/render -run TestGeneratedDatabase -v`; expect missing store output.**
- [ ] **Step 3: Implement a small tested key/value store.**

```go
func (s *Store) Get(ctx context.Context, key string) (string, error) {
    var value string
    if err := s.db.QueryRowContext(ctx, getSQL, key).Scan(&value); err != nil {
        return "", err
    }
    return value, nil
}
```

Use SQLite `?` and PostgreSQL `$1`/`$2` placeholders, never formatted SQL values.
`Put` uses a parameterized INSERT with
`ON CONFLICT (key) DO UPDATE SET value = excluded.value`.
`New` rejects nil; `Open` creates a lazy pool without contacting a live server;
`Ping` is explicit. Supply the schema under `sql/schema.sql` and document applying
it; do not mutate the user's database during init or auto-create tables at startup.
Generated main wires `store.Ping` into each selected component's readiness check
and closes its pool, handling close errors. HTTP health is independent of DB;
readiness reflects DB availability. Database-only projects still ship package tests.

- [ ] **Step 4: Run `go test ./internal/render -run TestGeneratedDatabase -v`; build/test both backends without external services, then execute PostgreSQL integration cases against a controlled test service.**
- [ ] **Step 5: Commit with `git add internal/render` and `git commit -m "feat: generate tested SQL database support"`.**

## Task 13: sqlc query generation and reproducibility

**Files:** Create `internal/render/sqlc.go`, `internal/render/sqlc_test.go`,
`internal/render/templates/sqlc/sqlc.yaml.tmpl`, `internal/render/templates/sqlc/queries.sqlite.sql.tmpl`,
`internal/render/templates/sqlc/queries.postgres.sql.tmpl`, `internal/render/templates/sqlc/store.go.tmpl`,
`internal/render/templates/sqlc/queries_test.go.tmpl`,
`internal/render/templates/sqlc/sqlite/db.go.tmpl`, `internal/render/templates/sqlc/sqlite/models.go.tmpl`, `internal/render/templates/sqlc/sqlite/queries.sql.go.tmpl`,
`internal/render/templates/sqlc/postgres/db.go.tmpl`, `internal/render/templates/sqlc/postgres/models.go.tmpl`, `internal/render/templates/sqlc/postgres/queries.sql.go.tmpl`;
modify `internal/render/database.go`, `internal/render/render.go`, and `internal/render/commands.go`.

**Interfaces:** Preserve the Store API from Task 12. Generated sqlc code lives in
`internal/store/queries`; `Store` delegates Get/Put to `queries.New(db)`.
The sqlc `sql_package` is `database/sql` for both backends, using the driver
registrations from Task 12. Tests cover both the facade and query package.

- [ ] **Step 1: Add a render test that regenerates sqlc output and compares bytes.**

```go
func TestSQLCRegenerationIsStable(t *testing.T) {
    for _, backend := range []string{"sqlite", "postgres"} {
        t.Run(backend, func(t *testing.T) {
            c := testproject.Config()
            c.Features.Database, c.Features.Access = backend, "sqlc"
            files, err := render.Files(c, "new")
            if err != nil { t.Fatal(err) }
            root := testproject.Write(t, files)
            before := testproject.File(t, files, "internal/store/queries/queries.sql.go")
            testproject.Go(t, root, "run", "github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1", "generate")
            after, err := os.ReadFile(filepath.Join(root, "internal/store/queries/queries.sql.go"))
            if err != nil { t.Fatal(err) }
            if !bytes.Equal(before, after) { t.Fatal("sqlc output drifted") }
        })
    }
}
```

Compare all generated query files, not only the representative file shown above.
Ship tests for Get/Put, construction, `WithTx`, and query failures; generated code
is not exempt from tests. SQLite runs against a temporary database; PostgreSQL
unit tests use sqlmock, with real integration tests when explicitly enabled.

- [ ] **Step 2: Run `go test ./internal/render -run TestSQLC -v`; expect missing sqlc assets.**
- [ ] **Step 3: Generate and bundle sqlc output during Rubric development, never during init.**

```sql
-- name: Get :one
SELECT value FROM entries WHERE key = $1;

-- name: Put :exec
INSERT INTO entries (key, value) VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = excluded.value;
```

The SQLite SQL input uses `?` placeholders. Both schemas are the key/value schema
from Task 12. Configure sqlc version `2`, output package `queries`, output path
`internal/store/queries`, schema `sql/schema.sql`, and queries `sql/queries.sql`.
Use the pinned sqlc to produce source assets before embedding; do not handwrite
files that claim to be sqlc output. Generated comment exceptions apply only to
sqlc-owned files; authored facade/test templates retain Rubric style rules.
Add `generate` to available commands only for sqlc projects.

- [ ] **Step 4: Run `go test ./internal/render -run 'TestSQLC|TestGeneratedDatabase' -v`; compare the full query directory before/after regeneration and run all shipped tests.**
- [ ] **Step 5: Commit with `git add internal/render` and `git commit -m "feat: generate tested sqlc projects"`.**

## Task 14: Mechanical style rules and a repo-local analyzer

**Files:** Create `internal/style/analyze.go`, `internal/style/tree.go`, `internal/style/analyze_test.go`,
`internal/style/tree_test.go`, `internal/style/sources.go`, `internal/style/sources_test.go`, `internal/style/repository_test.go`, `internal/style/main.go.tmpl`;
create `internal/render/style.go` and `internal/render/style_test.go`.

**Interfaces:** `style.Finding` contains `File string`, `Line int`, `Rule string`,
`Message string`. Produce `AnalyzeFile(string, []byte) ([]Finding, error)` and
`AnalyzeTree(string) ([]Finding, error)`. `style.Source` contains `Name string`,
`Data []byte`; `Sources() ([]Source, error)` exports the analyzer, thin main, and
tests for `.rubric/style/`. The style package does not import the renderer.

- [ ] **Step 1: Add boundary tests for comments and documentation.**

```go
func TestDocCommentLimit(t *testing.T) {
    for _, lines := range []int{2, 3} {
        source := "package sample\n" + strings.Repeat("// Example documents behavior.\n", lines) + "func Example() {}\n"
        findings, err := AnalyzeFile("example.go", []byte(source))
        if err != nil { t.Fatal(err) }
        found := false
        for _, item := range findings { found = found || item.Rule == "doc-lines" }
        if found != (lines == 3) { t.Fatalf("lines=%d findings=%v", lines, findings) }
    }
}
```

Add Unicode 150/151-character boundaries, missing exported docs, documented
exported fields/methods, declaration groups, interface methods, explanatory
function-body/trailing comments, valid Go directives/build tags, `_test.go`
test entry points, strings containing comment markers, generated sqlc headers,
malformed Go, unreadable files, and nested-module/vendor exclusions.

- [ ] **Step 2: Run `go test ./internal/style -v`; expect missing analyzer APIs.**
- [ ] **Step 3: Implement a deterministic AST analyzer using only the standard library.**

```go
func commentLineTooLong(line string) bool {
    return utf8.RuneCountInString(line) > 150
}
```

Parse comments with `go/parser`, associate declaration docs with AST nodes, and
count complete physical source lines. Exempt compiler/tool directives using
explicit recognized syntax, not all comments beginning with arbitrary words.
Use `ast.IsGenerated` for generator-owned source. Ignore test entry-point docs
only for valid Test/Benchmark/Fuzz/Example functions in `_test.go`; exported
application declarations remain checked. Classify non-doc comments inside
function bodies or trailing statements as forbidden explanatory comments.
Sort diagnostics by file, line, and rule; return parse/I/O failures separately.

Embed this package's analyzer source and its behavioral tests. In `Sources`,
replace only the parsed package-identifier byte span with `main`, so strings
inside tests are untouched. Include a thin stdlib flag-based `main.go` that calls
`AnalyzeTree`, prints findings, and exits nonzero on findings or operational error.
Generate the source assets only when linting is enabled, and ship their tests.
This makes the analyzer runnable with `go run ./.rubric/style .` without a globally
installed Rubric binary or a second application module. It needs no third-party
imports and does not modify an existing project's `go.mod`.
Bundle only `analyze.go`, `tree.go`, their corresponding tests, and the thin
main template. Use same-package tests so rewriting their package name creates
standalone tests without importing Rubric internals. Do not bundle the embedding
implementation or repository-specific tests. `TestRepositoryStyle` scans Rubric's
own production source through AnalyzeTree and fails on policy violations.

- [ ] **Step 4: Run `go test ./internal/style ./internal/render -run 'TestDoc|TestStyle|TestSources|TestComment|TestGeneratedAnalyzer' -v`; run the full copied suite with `go test ./.rubric/style` inside generated fixtures.**
- [ ] **Step 5: Commit with `git add internal/style internal/render` and `git commit -m "feat: generate enforceable Go style tooling"`.**

## Task 15: Independent Makefile, CI, linting, and skill options

**Files:** Create `internal/render/tooling.go`, `internal/render/tooling_test.go`, `internal/render/quote.go`,
`internal/render/quote_test.go`, `internal/render/templates/tooling/Makefile.tmpl`, `internal/render/templates/tooling/check.sh.tmpl`,
`internal/render/templates/tooling/golangci.yml.tmpl`, `internal/render/templates/tooling/workflow.yml.tmpl`, `internal/render/templates/tooling/workflow-skill.md.tmpl`,
`internal/render/templates/tooling/testing-skill.md.tmpl`, and `internal/render/templates/tooling/style-skill.md.tmpl`; modify command/guidance rendering.

Modified renderer files are `internal/render/render.go`, `internal/render/commands.go`,
and `internal/render/guidance.go`.

**Interfaces:** `Tooling(config.Config) ([]File, error)` returns files from the
same normalized commands as guidance. `QuotePOSIX(string) string` quotes literal
arguments; a separate `QuoteMake(string) string` doubles `$` before the value
enters a Make recipe. Shell text is a serialization boundary, never stored as an
unparsed command in `rubric.yaml`.

- [ ] **Step 1: Test all 16 tooling-toggle combinations and literal arguments.**

```go
func TestShellQuotePreservesLiteralText(t *testing.T) {
    value := "a path with 'quotes' $HOME `printf wrong`"
    command := exec.Command("sh", "-c", "printf '%s' " + QuotePOSIX(value))
    output, err := command.Output()
    if err != nil { t.Fatal(err) }
    if string(output) != value { t.Fatalf("got %q", output) }
}
```

Fixtures check Actions without Makefile, Makefile without linting, no optional
tooling, module-only no-packages success, copied analyzer tests, plain test-command
failures, a broken `go list`, changed formatting, and sqlc-only generation tooling.
Assert no implicit toggles, no references to absent targets, no commands for
deferred Rubric features, and no credentials copied from the process environment.
Run shell syntax checks and parse workflow YAML in tests, not just string-match it.

- [ ] **Step 2: Run `go test ./internal/render -run 'TestTooling|TestShellQuote|TestEmptyChecks' -v`; new artifacts must fail their assertions first.**
- [ ] **Step 3: Render shared check scripts and thin Make/Actions wrappers.**

```go
func QuotePOSIX(value string) string {
    return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
```

Use `[[` and `]]` as Go template delimiters for workflow templates to avoid
consuming GitHub `${{ ... }}` expressions. Quote structured arguments for their
specific shell/YAML/Make boundary; do not reuse JSON quoting as shell quoting.
All script paths are relative to the selected project root.

The generated `.rubric/check.sh` implements explicit `build`, `test`, `lint`,
`generate`, and named run operations that exist in the command inventory. Before
package checks, capture `go list ./...` with status preserved: a discovery failure
fails the check, while an empty successful result reports no application packages.
The `test` operation also runs `go test ./.rubric/style` when the analyzer exists,
including when the application itself has no packages. Do not let the empty-app
branch skip analyzer tests. Scripts use `set -eu` and `exec` for a selected run
operation. Generated scripts are source-controlled; tooling remains optional.

Linting runs formatting/import checks, `errcheck`, `govet`, `staticcheck`, `revive`,
and the local analyzer. Use golangci-lint configuration schema `version: "2"`,
`linters.default: none`, and explicit enabled linters. Use the v2 formatter
configuration for gofmt/goimports and verify a deliberately unformatted fixture
causes the check to fail. Check formatting without rewriting via
`go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 fmt --diff`.
Launch linting with
`go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run` and sqlc
with `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`, without adding
either to application `go.mod`.

Render Make targets through the same check script; render Actions either through
Make or the script depending on the independent toggle. CI uses the immutable
Actions refs above and a toolchain at least as new as both the preserved project
Go directive and `generator.go`. Diagnose an incompatible local toolchain before
executing a generated check, without rewriting the application's `go.mod`.
PostgreSQL projects
add an isolated `postgres:17.6` test service, a database named `rubric_test`, and
an explicitly enabled integration-test step. For that disposable loopback-only
test service use trust authentication and a credential-free
`TEST_DATABASE_URL=postgres://postgres@127.0.0.1:5432/rubric_test?sslmode=disable`.
Never configure or start a real application database during initialization.

Skills are generated as `.agents/skills/rubric-workflow/SKILL.md`,
`.agents/skills/rubric-testing/SKILL.md`, and `.agents/skills/rubric-style/SKILL.md`. Give each YAML frontmatter
with `name` and scoped `description`, actual repo commands, and the mandatory
test/style rules. Apply the writing-skills/skill-creator instructions when creating
these skill templates during execution. Style advice can exist without linting;
do not falsely say lint has been installed. Include all three source-guide links.

- [ ] **Step 4: Run `go test ./internal/render -run 'TestTooling|TestShellQuote|TestEmptyChecks|TestGeneratedAnalyzer' -v`; execute representative generated checks with and without Make installed.**
- [ ] **Step 5: Commit with `git add internal/render` and `git commit -m "feat: generate optional project tooling"`.**

## Task 16: Bubble Tea wizard over the shared service

**Files:** Create `internal/wizard/model.go`, `internal/wizard/target.go`, `internal/wizard/application.go`,
`internal/wizard/tooling.go`, `internal/wizard/review.go`, `internal/wizard/apply.go`, `internal/wizard/view.go`, `internal/wizard/model_test.go`, and
`internal/wizard/flow_test.go`; modify `internal/cli/command.go`, `internal/cli/cli_test.go`, `go.mod`, and `go.sum`.

**Interfaces:** `wizard.New(context.Context, initialize.Request, Backend) Model` and
`wizard.Run(context.Context, initialize.Request, io.Reader, io.Writer) (Outcome, error)`.
`Backend` contains `Prepare func(context.Context, initialize.Request) (plan.Plan, error)`
and `Apply func(context.Context, initialize.Request, plan.Plan) (write.Result, error)`.
`Outcome` contains `Request initialize.Request`, `Plan plan.Plan`, `Result write.Result`,
and `Cancelled bool`. `Model` uses the Bubble Tea v2 methods from Task 10.

- [ ] **Step 1: Test that cancellation cannot invoke apply.**

```go
func TestCancelNeverApplies(t *testing.T) {
    applied := false
    backend := Backend{
        Prepare: initialize.Prepare,
        Apply: func(context.Context, initialize.Request, plan.Plan) (write.Result, error) {
            applied = true
            return write.Result{}, nil
        },
    }
    model := New(t.Context(), initialize.Request{Target: t.TempDir(), Mode: "auto"}, backend)
    next, command := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
    if command != nil { command() }
    if applied || !next.(Model).outcome.Cancelled { t.Fatal("cancellation applied changes") }
}
```

Cover all five stages, back navigation with retained values, clearing database
access after disabling the database, Viper without Cobra, both empty starters,
existing unknown-library facts, invalid module input, zero-size terminals,
partial field edits, duplicate key events while preparing, stale async responses,
per-file replace/skip decisions, apply errors, and cancellation during apply.
Drive complete flows and compare their resolved config/files to unattended mode.

- [ ] **Step 2: Run `go test ./internal/wizard ./internal/cli -v`; expect missing wizard APIs and interactive routing.**
- [ ] **Step 3: Implement explicit stages and asynchronous engine commands.**

```go
type stage int

const (
    targetStage stage = iota
    applicationStage
    toolingStage
    reviewStage
    applyStage
    doneStage
)

func (m Model) View() tea.View {
    return tea.NewView(m.pageText())
}
```

`pageText() string` renders the current stage and validation errors using width
and height clamped to safe minimums. Keep text fields, selection lists, and page
state in the model; template generation and filesystem changes stay in Backend.
Include all Request flags through wizard controls, including editable existing
commands/entry points. Do not require a browser or graphical companion.
Each prepare command carries a monotonically increasing request ID; Update
ignores superseded results. Only a review-stage apply action can invoke Apply.
Skipping optional tooling reparses selections and prepares a new reviewed plan;
replacement decisions refer to the exact reviewed snapshot. Show the file diff
and user-authored AGENTS content without requiring semantic prose analysis.

Install Bubble Tea and terminal dependencies at the pins above. CLI selects the
wizard only for an actual terminal, text format, and absence of
`--non-interactive`. JSON and redirected output always stay unattended. Use
Bubble Tea's context and I/O options; restore terminal state after every result.
Display progress and actionable recovery paths, and never claim that merely
generating a project has run its application tests or downloaded dependencies.

- [ ] **Step 4: Run `go test -race ./internal/wizard ./internal/cli ./internal/initialize`; compare interactive/unattended outcomes.**
- [ ] **Step 5: Commit with `git add internal/wizard internal/cli go.mod go.sum` and `git commit -m "feat: add Bubble Tea initialization wizard"`.**

## Task 17: Full acceptance matrix, CI, coverage, and user documentation

**Files:** Create `internal/acceptance/matrix_test.go`, `internal/acceptance/existing_test.go`,
`internal/acceptance/postgres_test.go`, `internal/acceptance/terminal_unix_test.go`, `internal/acceptance/coverage_test.go`,
`.github/workflows/ci.yml`, `.golangci.yml`, `.gitignore`, and `docs/cli-init.md`;
modify `README.md`, `internal/testproject/project.go`, `internal/testproject/project_test.go`, `go.mod`, and `go.sum`.

**Interfaces:** Reuse the public CLI/service and testproject APIs. Add test-only
`testproject.Snapshot(t *testing.T, root string) map[string][]byte` and
`testproject.AssertBehaviorCoverage(t *testing.T, root, profile string)`.
The latter checks source files against a coverage profile, ignoring only
`main.go` and path segments named `cmd`; it reports uncovered functions/branches
for review without inventing a percentage threshold.

- [ ] **Step 1: Add a generated-test inventory assertion and matrix harness.**

```go
func TestCatalogMatrixSize(t *testing.T) {
    if got := len(catalog.Cases()); got != 180 {
        t.Fatalf("catalog cases=%d, want 180", got)
    }
}
```

Enumerate both starters whenever there is no named executable, including
database-only and Viper-only projects. Ten of the 180 feature sets have no HTTP,
CLI, or TUI executable, so this gives 190 application configurations. For each,
generate through the real CLI, perform documented
`go mod tidy`, and build/test the result. Render all 16 tooling combinations;
source-equivalent outputs may share a compiled application check, but execute
the check commands from every distinct script/workflow variant in isolated
fixtures and separately test its optional helper code. Validate workflow YAML
and pinned action references locally; a live GitHub run occurs when the branch
is integrated through the user's chosen workflow. Use `RUBRIC_ACCEPTANCE=1` to enable expensive subprocess/database/PTY tests
and fail on missing required infrastructure when enabled. Fast tests run by
default; repository CI must enable all acceptance suites before release.

Record expected non-exempt source files from the rendered file inventory. Ensure
each belongs to a package with real shipped tests and has exercised behavior;
then inspect coverage for untested decisions. Check copied analyzer source/tests
explicitly because `go test ./...` excludes dot directories. Exclude data-only Go
files with no executable statements from statement-coverage matching, but still
exercise their values through package tests. Generated sqlc files are not blanket
excluded. Existing-project fixtures assert preservation of all original source,
tests, module files, user instructions, and conflicting configuration.

- [ ] **Step 2: Run `go test ./internal/acceptance -run 'TestCatalogMatrixSize|TestGeneratedTestInventory' -v`; add deliberately missing generated tests to fixture expectations and observe the gate reject them before fixing the templates.**
- [ ] **Step 3: Implement matrix execution, coverage checks, and terminal integration.**

```go
func runProjectChecks(t *testing.T, root string) {
    t.Helper()
    testproject.Go(t, root, "mod", "tidy")
    testproject.Go(t, root, "build", "./...")
    testproject.Go(t, root, "test", "-coverprofile=coverage.out", "./...")
    testproject.AssertBehaviorCoverage(t, root, filepath.Join(root, "coverage.out"))
}
```

Call that helper only when application packages exist; validate module-only
output without manufacturing Go tests. Main-only output builds but has no
non-exempt application behavior coverage requirement. Give subprocesses bounded
contexts and include their exact arguments/output in failures. Shard the 190
application cases across four CI jobs using stable case indices; never use a
random sample in place of the accepted complete capability matrix.

Use the pinned `creack/pty` to run the built Rubric binary in a pseudo-terminal
on supported Unix CI, send Ctrl-C, and verify exit 130, terminal restoration,
and an unchanged destination. Ordinary wizard model tests remain portable.
Database acceptance enables generated integration-tagged tests against the
isolated PostgreSQL service and verifies real Get/Put/transaction behavior for
both access modes. Run SQLC regeneration, no-op initialization, stale-preview,
rollback, and literal-path cases. Test the documentation's command examples as
CLI scenarios; use temporary directories rather than the repository as targets.

CI runs fast tests with race detection, vet, style/lint checks, acceptance shards,
and generated database integration tests. Use `.golangci.yml` with the same
effective policy as generated lint configuration, and run `TestRepositoryStyle`
for Rubric's own source. Keep Go and Actions pins aligned with the catalog.
Retain coverage profiles in the job workspace and print their `go tool cover
-func` summaries in job logs, giving a persistent report without another Action
dependency.
Do not mark source-equivalent matrix cases as untested skips; report their shared
compiled result and separately verified artifact variants.

Document local installation as `go install ./cmd/rubric` and local development
as `go build -o ./bin/rubric ./cmd/rubric`. Do not advertise an unpublished remote
release. User docs include the full
flag table, both minimal starters, every capability, optional tooling, config
precedence, headless JSON examples, dependency setup, missing-package behavior,
test commands, PostgreSQL integration setup, conflicts, ownership, and recovery.
Ignore `/bin/`, coverage outputs, and local acceptance artifacts in `.gitignore`.
Keep the README roadmap for deferred commands clearly distinct from implemented v1.

- [ ] **Step 4: Run the final verification commands below and inspect every result.**

```sh
go test -race -coverprofile=/tmp/rubric-unit-coverage.out ./...
go vet ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 fmt --diff
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run
go build -o /tmp/rubric ./cmd/rubric
RUBRIC_ACCEPTANCE=1 go test -timeout 45m ./internal/acceptance -v
git diff --check
```

Provide `TEST_DATABASE_URL` for the isolated test database before the acceptance
command; no production database is used. Execute the same selected formatting,
lint, and analyzer checks described by the generated tooling. Confirm Rubric's
coverage report covers all substantive packages and explain any uncovered
behavior before calling the work complete. A failing or unrun acceptance suite
is a remaining requirement, not evidence that v1 is finished.

- [ ] **Step 5: Commit with `git add internal/acceptance internal/testproject .github/workflows/ci.yml .golangci.yml .gitignore docs/cli-init.md README.md go.mod go.sum` and `git commit -m "test: verify rubric and generated projects end to end"`.**

## Spec coverage and final review

| Approved acceptance criterion | Owning tasks |
| --- | --- |
| 1. Both minimal starters | 3, 7, 15, 17 |
| 2. All catalog options/combinations build and test | 3, 8–13, 17 |
| 3. Separate composable executables | 8–12, 17 |
| 4. Viper independent of Cobra; database independent of HTTP | 11–13, 17 |
| 5. Mandatory tests in Rubric and generated non-exempt code | Every task; 14 and 17 verify helper/generated coverage |
| 6. Equivalent input modes | 1, 7, 16–17 |
| 7. Dry-run/cancellation preservation | 5–7, 16–17 |
| 8. Clear headless validation/conflict failures | 1, 5, 7, 17 |
| 9. Existing-project preservation | 2, 5–7, 17 |
| 10. No-op reruns and edited-content conflicts | 5–7, 17 |
| 11. Accurate AGENTS instructions | 4, 7–15, 17 |
| 12. Style policy and mechanical checks | 4, 14–15, 17 |
| 13. Independent tooling with real commands | 4, 15, 17 |
| 14. Write failure and recovery | 6–7, 16–17 |

After task completion, use `superpowers:requesting-code-review` for the selected
execution workflow's independent review, resolve findings, and use
`superpowers:verification-before-completion` before claiming success. Integration
or branch finishing follows `superpowers:finishing-a-development-branch` and the
user's selected action; this plan itself does not authorize a remote push or merge.

## Execution handoff

Recommended execution: **native in this session**, with one independent review
of the complete branch. The tasks share configuration, file-plan, and template
contracts; keeping their implementation context together reduces handoff drift.
Subagent-driven execution remains an option for per-task independent reviews,
with additional context cost. The user reviews this plan and selects the method
before implementation begins.
