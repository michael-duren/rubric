# Rubric init v1 design

Date: 2026-09-24

Status: Approved by the user on 2026-09-24, including the mandatory testing
requirement. Implementation awaits review of the written implementation plan
and selection of its execution method.

## Purpose and success criteria

`rubric init` helps developers start a Go project or introduce Rubric into an
existing Go repository. People use a Bubble Tea wizard; agents and scripts can
supply the same choices through configuration and flags without a terminal.

New projects can combine independently selected capabilities, or start almost
empty. Existing projects receive configuration, tooling, and guidance describing
the application already present. Generated `AGENTS.md` instructions must match
the actual project rather than describe an assumed stack.

A successful v1 produces a reviewable, reproducible set of files, preserves
existing work, documents commands that exist, and includes meaningful tests for
Rubric and for the application code it generates. After the documented dependency
setup, runnable generated projects must build and their default tests must pass.

## Agreed scope

- Implement `rubric init` with a Bubble Tea wizard and noninteractive operation.
- Support new Go projects and existing repositories containing one Go module.
- Offer composable HTTP, database, CLI, TUI, and configuration capabilities.
- Offer both a module-only starter and a minimal runnable starter.
- Use curated library choices, with all application capabilities optional.
- Offer repo-local agent skills, linting, a Makefile, and GitHub Actions as
  independently selectable development tooling.
- Generate project-aware agent guidance and persist choices in `rubric.yaml`.
- Provide previews, explicit conflict handling, and repeatable initialization.
- Test all Rubric behavior and ship tests with generated Go code, except
  `main.go` files and files beneath any `cmd/` directory.

The approved repository scope is new and existing Go projects. Limiting each
invocation to one module is a proposed v1 implementation boundary.

Multi-module workspaces, adding application components to existing projects,
third-party template plugins, automatic source migrations, deployment tooling,
issue-filing automation, advanced path coverage, performance measurement, and
LSP-diagnostic collection are outside v1. The later commands `rubric validate`,
`rubric make`, `rubric paths`, and `rubric perf` are outside this implementation.
Generated tooling must not call those commands before they exist.

The rest of this document specifies implementation defaults for the approved
scope. They can be adjusted during written-spec review.

## Capability catalog

| Capability | Choices | Conditions |
| --- | --- | --- |
| HTTP | None, standard-library `net/http`, Chi | One HTTP implementation per project. |
| Database | None, SQLite, PostgreSQL | One database backend in v1; independent of application type. |
| Database access | Handwritten SQL using `database/sql`, sqlc | Asked only when a database is selected. |
| CLI | None, standard-library `flag`, Cobra | Independent of HTTP and TUI. |
| TUI | None, Bubble Tea | Independent of HTTP and CLI. |
| Configuration | Standard-library handling, Viper | Viper is optional and does not require Cobra. |

Start with no application capabilities selected. When a user enables a capability,
suggest `net/http`, SQLite, handwritten SQL, or `flag` for its respective choice;
the user can select the other supported option. Do not silently enable HTTP,
a database, or a CLI when another component is selected. Library dependencies
necessary for an explicitly selected component appear in the final preview.

Rubric bundles tested dependency and tool versions with its templates. Record the
Rubric version, template revision, supported Go version, and selected tools in
the saved configuration. Do not resolve `latest` during generation. The initial
supported Go baseline follows this repository's `go.mod` (`1.26.7`); preserve the
declared Go version of existing projects and report tooling incompatibilities
without upgrading their module automatically.

## New-project output

Every new project receives `go.mod`, `README.md`, `rubric.yaml`, `AGENTS.md`, and
the small Rubric ownership manifest described below. Optional tooling adds only
its own required files.

With no HTTP, CLI, or TUI executable selected, users choose either:

1. **Minimal runnable (default):** add a thin `cmd/<name>/main.go`, where `<name>` is
   the last module path element made path-safe (`app` when nothing usable remains). It
   loads settings and opens the store when those are selected; no test is required for
   that file. If behavior grows beyond entry-point wiring, place it in a tested package.
2. **Module-only:** no application Go files and no empty test files.

(Amended 2026-09-24 at the user's request: every generated project has an executable
under `cmd/` unless module-only is explicitly chosen. Existing projects adopted without
a saved starter keep module-only semantics, so guidance never describes user code as
Rubric scaffolding.)

Selecting executable components creates separate binaries:

| Component | Entry point | Behavior and tests |
| --- | --- | --- |
| HTTP | `cmd/server/main.go` | `internal/httpserver/` |
| CLI | `cmd/cli/main.go` | `internal/cli/` |
| TUI | `cmd/tui/main.go` | `internal/tui/` |
| Database | No separate executable | `internal/store/` |
| Viper | No separate executable | `internal/config/` |

Only selected directories exist. Components share packages under `internal/`;
they do not import each other's `main` packages. A CLI or TUI selection does not
force an HTTP server. Executables contain wiring, startup, shutdown, and process
exit handling; parsing, rendering, handlers, and other substantive behavior live
in tested packages outside `cmd/`.

When executable components are selected, they replace the root minimal entry
point in the initial generation plan. Database or configuration support can also
be selected without an executable, producing reusable packages and their tests.
The preview makes that distinction explicit.

Templates provide small working examples: an HTTP health endpoint, a basic CLI
command, a Bubble Tea model that can exit cleanly, and database connection and
query examples. sqlc includes its configuration, SQL inputs, generated Go, and
tests exercising the query code. Viper includes explicit configuration loading
and tests for defaults, supplied files, environment values, and invalid input.
Generated code must use the selected integrations rather than only adding unused
dependencies. Runtime settings and environment-variable names are documented;
credentials are never baked into source or `rubric.yaml`.

## Existing-project behavior

Inspect `go.mod`, Go imports and package declarations, executable entry points,
and known configuration files. Detection is read-only and does not execute
application code. Dependencies alone are evidence of availability, not proof
that a component is active. Show evidence and allow users to correct inferred
components, paths, and commands before generation.

An existing project can use libraries outside the generation catalog. Describe
them when reliably detected without replacing them or claiming unsupported
library-specific integration. Unknown facts remain explicitly unknown; they
must not become invented agent instructions. Users may supply missing commands
or leave optional run targets out.

Existing projects receive Rubric configuration, agent guidance, and selected
tooling. Do not scaffold application packages, rewrite `go.mod`, replace tests,
upgrade dependencies, or reformat application code. Existing source may fail
newly selected lint rules; report that result without silently fixing it.

Operate on the chosen module directory. If an invocation covers multiple modules
or only a `go.work` workspace, require a specific module directory rather than
guessing. A nonempty directory without a Go module is not automatically a new
project: the wizard requires an explicit new-project choice; unattended use
requires `--mode new`. Existing files still receive normal conflict handling.

## CLI and wizard contract

The command shape is `rubric init [directory]`; the default target is the current
directory. `--mode auto|new|existing` selects detection or an explicit mode.

The wizard has five stages:

1. **Target:** show the destination and detected mode; collect a module path and
   project description for new projects, or review detection for existing ones.
2. **Application:** select new-project capabilities or correct existing-project
   facts. Show dependent questions only while their parent capability is enabled.
3. **Tooling:** choose skills, linting, Makefile, and GitHub Actions. Explain which
   files each option adds and allow all optional tooling to be disabled.
4. **Review:** show final selections and create/update/unchanged/conflict actions,
   with file-content previews. Back navigation retains answers that still apply.
5. **Apply:** write the approved plan and display created files, setup commands,
   and available build, test, lint, and run commands.

Support keyboard navigation, inline validation errors, terminal resizing, and
cancellation. Until apply begins, cancellation leaves the destination unchanged.
Do not print the full-screen TUI into redirected output or start it without a
usable terminal. Basic help and errors must remain readable without color.

Support these controls in addition to capability and tooling flags:

- `--non-interactive`: never ask questions or launch the TUI.
- `--config <path>`: read a Rubric configuration document as input.
- `--dry-run`: perform validation and rendering, then print or display the plan
  without creating directories, writing files, or downloading dependencies.
- `--format text|json`: select a final report format; JSON reports contain mode,
  resolved choices, file actions, conflicts, diagnostics, and next commands.
  Selecting JSON implies noninteractive operation.

Expose flags for every wizard choice, including module path, minimal starter,
capability selections, and each tooling toggle. Explicit flags override input
configuration; input configuration overrides saved target settings; detection
fills missing existing-project facts; documented defaults apply last. Explicit
disabled values and empty lists must not be replaced by defaults.

Flags and configuration are decoded into the same typed model as wizard answers.
Reject unknown keys, unsupported values, incompatible selections, malformed
module paths, and unsupported schema versions. Required missing values fail
before writes in unattended mode. Non-terminal invocations follow these same
rules instead of falling back to prompts. No general `--force` overwrite option
is needed in v1.

A new project requires a module path; its display name defaults to the last path
component and its description is optional. Other selections use the documented
defaults unless supplied. With no executable selected, default to the minimal
runnable starter. Tooling toggles default to disabled; the wizard makes their
availability explicit, and unattended use has the same defaults.

Exit status is 0 for successful application or a valid conflict-free dry run,
2 for invalid input or unresolved conflicts, 1 for operational failure, and 130
for cancellation. In JSON mode emit one complete report on stdout and route
incidental logs to stderr; do not mix a TUI with JSON output.

## Configuration and generation architecture

Use bundled, composable templates. A wrapper around Go Blueprint would require
adapting its output, component boundaries, and guidance; external template
plugins would add an additional compatibility and distribution contract. Both
are deferred in favor of a single tested catalog owned by Rubric.

Keep these responsibilities distinct:

| Unit | Responsibility | Dependencies |
| --- | --- | --- |
| Command adapter | Parse arguments and select interactive or unattended use | Configuration, wizard, initializer |
| Wizard | Collect and validate answers; display preview and progress | Shared configuration and plan types |
| Detector | Produce project facts and evidence | Read-only filesystem and Go source/module parsing |
| Configuration | Merge inputs, validate choices, normalize persisted state | Versioned schema and capability catalog |
| Planner and renderer | Produce file contents and next commands | Normalized configuration and bundled templates |
| Writer | Check conflicts and apply file actions | Rendered plan, filesystem, ownership manifest |
| Reporter | Produce human-readable or JSON results | Shared plan and result types |

The flow is inputs and detection → normalized configuration → rendered plan →
preview and conflict resolution → apply → result. The wizard does not contain a
second generator, and templates do not rediscover the repository themselves.
In particular, Makefile and agent-guidance generation use the saved project model.

`rubric.yaml` contains a schema version, module identity, Go version, project
description, detected or selected components, entry-point paths, build/test/run
commands, tooling selections, style-policy identifier, and generator/template
versions. Persist explicit disabled choices. File hashes and ownership metadata
belong in `.rubric/manifest.json`, separate from user-editable configuration.
The invocation destination is an input, not a machine-specific absolute path
committed into portable project configuration.

The same normalized configuration and template revision must produce the same
managed file contents. Avoid timestamps, random identifiers, host-specific paths,
and environment-dependent dependency selection in generated content. Dependency
download and checksum resolution are a documented post-generation setup step;
generation and dry runs themselves do not execute package installation. Tests
must run that documented setup before asserting that generated projects build.

## File ownership, conflicts, and recovery

The review plan lists each affected path and why it changes. Distinguish complete
Rubric-owned files from managed sections inside user-owned documents.

- Append a clearly marked managed section to an existing `AGENTS.md`, preserving
  all surrounding content. A new `AGENTS.md` also uses this boundary so users can
  add their own instructions outside it.
- Store hashes of the last written managed content in `.rubric/manifest.json`.
  Replace managed content on rerun only if it still matches that version.
- Treat `rubric.yaml` as user-editable input, not immutable generated content.
  Load and validate its current contents, preserve its comments when updating
  supported fields, and compare against the invocation's initial snapshot to
  detect concurrent edits. Normal configuration edits do not trigger the stale
  generated-file conflict rule. Do not include the ownership manifest in its own
  content-hash inventory.
- Leave identical files unchanged. Never replace a colliding user-owned file or
  an edited managed section silently. Show its proposed diff and require an
  explicit per-file resolution in the wizard.
- Unattended conflicts fail before any writes and are listed in the report.
  Resolve them outside the command, then rerun. A skipped optional artifact must
  also be omitted from documented commands and dependent tooling; mandatory
  config or guidance conflicts prevent completion.
- Application source templates are initial scaffolding, not continuously
  regenerated files. On later runs, classify the repository as existing, refresh
  facts, and preserve application sources and tests.

Render and validate all output before applying it. Restrict generated paths to
the selected destination and refuse symlink redirection for managed write
targets. Recheck conflict assumptions immediately before replacement. Stage
content before individual file replacements and record applied actions so a
failure can restore files written by this invocation when safe. Do not claim
cross-file atomicity: if restoration is incomplete, list affected paths and
recovery instructions in the failure report.

Do not delete obsolete files automatically. Report them for review and stop
referring to them as active features. Never remove files that changed after the
plan was prepared. A second unchanged initialization is a no-op.

## Agent guidance and development tooling

The managed `AGENTS.md` section includes:

- The stated project purpose and selected or verified stack.
- Existing component directories and executable entry points.
- Exact setup, build, test, lint, generation, and run commands that are available.
- Configuration sources and required environment-variable names.
- The mandatory testing policy, style references, and Rubric-specific overrides.
- Paths to selected repo-local skills and instructions to report measured check
  results rather than claim unexecuted validation succeeded.

Write only applicable guidance. Module-only output must not advertise a server,
database, run command, or implemented application tests. Show existing
user-authored guidance alongside proposed changes so the user can identify
contradictions. Preserve that guidance; v1 does not attempt semantic conflict
detection or automatic rewriting of arbitrary prose.

When selected, install Rubric's own workflow skills under
`.agents/skills/rubric-*/SKILL.md` and reference them explicitly from `AGENTS.md`.
Limit v1 skills to the available project workflow, testing, and style review.
They must not instruct agents to call deferred Rubric commands. Collision rules
apply to existing skill files just as they do to other output.

Generate Makefile targets for the commands actually present: build, test, lint
when enabled, sqlc generation when selected, and separate run targets for each
executable. A module-only project without packages must not receive a CI job
that fails just because there is no Go source yet; report that there are no
packages to check and begin checking automatically once source is added.

GitHub Actions invokes the same configured checks, through Make targets when a
Makefile is enabled or directly otherwise. Selecting Actions does not require a
Makefile or silently select linting. Pin tool dependencies and Actions references
in the generated workflow. Tool launchers resolve pinned tools through the Go
toolchain without requiring a globally installed lint or generation binary.

## Style policy

Use these references:

1. [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).
2. The [Formatting and style section of Peter Bourgon's Go: Best Practices for
   Production Environments](https://peter.bourgon.org/go-in-production/#formatting-and-style).
3. The [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).

Generate a short local style policy that states the effective rules and links to
these sources. Explicit Rubric/project overrides take precedence, followed by Go
Code Review Comments, Bourgon's specified section, and Uber's additional guidance
where compatible. Formatting follows Go's formatter; do not turn soft line-length
advice into an unrelated hard source-code limit.

The agreed overrides are:

- Agents do not add explanatory inline or trailing comments in Go code.
- Exported application symbols require documentation comments.
- A declaration's documentation comment has at most two physical lines, each
  containing at most 150 Unicode characters, including its comment delimiter.

Comments required by the language or tools, such as build constraints and Go
directives, are not explanatory comments or declaration documentation. Recognized
third-party generated code retains its generator-owned comments; normal Rubric
application templates and agent-authored code follow the policy. Test functions
are not exported application API merely because their names start with `Test`.

Linting must enforce what can be checked mechanically, including exported docs
and the documentation length limit. Include a focused analyzer for requirements
the selected standard linters cannot enforce. A tool cannot identify whether a
comment was written by an agent, so the ordinary-inline-comment check applies
consistently to authored source regardless of author. Document exclusions and
required directives rather than treating all comments identically.

Use formatter/import checks and a curated lint configuration covering error
handling, vet diagnostics, static analysis, and naming/style issues. Contextual
design advice remains in agent guidance; passing lint is not a claim that every
subjective recommendation in all three guides has been mechanically proved.
The templates themselves must satisfy this policy even when a user disables
lint-tool generation.

## Mandatory testing in v1

Testing is part of the product deliverable, not an optional scaffold feature.
Disabling Makefile, linting, skills, or CI does not remove generated tests.
The advanced changed-path coverage engine remains deferred; that does not defer
ordinary unit, integration, or generation tests.

### Rubric's own code

Cover all Rubric behavior with meaningful automated tests: parsing and precedence,
configuration validation, detection, wizard transitions, template composition,
agent guidance, the style analyzer, file ownership, conflict handling, writing,
cancellation, error reporting, and CLI exit behavior. Thin command entry points
can be exercised through command integration tests; moving logic into `cmd/`
does not remove the requirement to test Rubric's behavior.

Keep unit tests deterministic. Use temporary directories for filesystem behavior
and injected boundaries for operational failures. Include end-to-end CLI tests
for interactive-state equivalence with unattended input, dry runs, repeat runs,
user edits, malformed configuration, and partial-write recovery. Exercise Bubble
Tea model updates without requiring a real terminal for every test, and include
a terminal-level smoke check for startup and cancellation.

### Tests shipped in generated projects

Every generated Go package containing non-exempt code includes meaningful tests
that exercise that code. The only source-location exemptions are files named
`main.go` and files under a directory named `cmd`. These exemptions allow thin
entry points; they are not a place to hide handlers, configuration parsing,
database logic, commands, or TUI behavior from testing.

A separate `_test.go` file for every source file is not necessary; package tests
must cover the behavior of every non-exempt generated source file. No placeholder
assertions or permanently skipped tests count as satisfying this requirement.
sqlc output is exercised through tests of its generated query behavior; being
machine-generated is not a testing exemption. Existing application files are not
newly generated and are not automatically rewritten to add tests during init.

Include relevant success, invalid-input, failure, and cancellation cases for each
component. HTTP tests use an in-process server or recorder, CLI tests exercise
parsing and output, TUI tests exercise update/state behavior, and configuration
tests isolate environment and filesystem input. SQLite tests use a temporary
database. PostgreSQL templates include tests of their application-side behavior
without a live server and separate integration tests for real database behavior.
CI selected with PostgreSQL provisions the database for those integration tests;
without CI, document the explicit environment and command for running them.

Default `go test ./...` for a generated project with packages must not depend on
credentials, a live database service, or a user's terminal. Real PostgreSQL
integration tests are explicitly enabled; ordinary tests cannot all be skipped
when that service is absent. Report unit and integration results separately.

### Testing the generator's output

For every supported valid capability combination, generate a temporary project,
perform the documented dependency setup, and verify build and test success. The
test suite can shard and cache dependency downloads, but testing each template
in isolation does not replace checking their supported compositions.

Check both minimal starters explicitly. A module-only project is validated as a
module with no application packages; a `cmd/<name>`-only starter is built and does
not require an artificial test. For all other combinations, confirm that
non-exempt code has behavioral tests. Run selected linting and any sqlc generation
checks, and verify that regeneration is stable. Exercise real PostgreSQL queries
in generator integration tests using a controlled database service.

Use focused content assertions or snapshots for generated configuration and
guidance, plus actual builds and tests for generated Go behavior. Collect coverage
for Rubric and generated non-exempt code so missing tested behavior is visible.
This spec does not invent a numerical coverage percentage or equate line coverage
with proof that every execution path was exercised.

## Acceptance criteria

1. Both minimal starters work with all optional development tooling disabled.
2. Each catalog option and every supported valid combination generates a project
   that builds and passes its applicable tests after documented dependency setup.
3. HTTP, CLI, and TUI can coexist as separate executables with shared internal code.
4. Viper is selectable with or without Cobra; database support does not require HTTP.
5. Every generated non-exempt Go source file has behavior covered by shipped tests,
   independently of Makefile or CI selection. Rubric's own behavior is tested too.
6. Equivalent wizard answers, flags, and configuration produce equivalent output.
7. Dry runs and cancellation before apply leave the target unchanged.
8. Missing unattended inputs and unresolved conflicts fail before writes with
   actionable text or structured JSON diagnostics and the specified exit status.
9. Existing-project initialization preserves application code, tests, dependencies,
   user-authored guidance, and colliding configuration unless explicitly resolved.
10. Unchanged reruns are no-ops; edited managed content is identified as a conflict.
11. Generated `AGENTS.md` accurately reflects files, libraries, executables, commands,
    tests, and tooling that are actually present.
12. Generated templates follow the selected style policy, and selected linting
    enforces the explicit mechanical comment requirements.
13. Optional Makefile and CI choices work independently and never call unfinished
    Rubric commands or assume absent application packages.
14. Injected write failures report what happened and preserve or recover prior
    content according to the documented recovery contract.

## References

- [Go Blueprint](https://github.com/melkeydev/go-blueprint): reference for selectable
  project scaffolding; not a required runtime dependency.
- [Bubble Tea](https://github.com/charmbracelet/bubbletea): Rubric's wizard and the
  generated TUI option.
- [Chi](https://github.com/go-chi/chi), [Cobra](https://github.com/spf13/cobra),
  [Viper](https://github.com/spf13/viper), and [sqlc](https://github.com/sqlc-dev/sqlc):
  optional generated-project integrations.
- [Go database documentation](https://go.dev/doc/database/): standard SQL access.
