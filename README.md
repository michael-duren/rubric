# rubric

## about

Agentic skills and deterministic tooling that grade agent work against a strict,
explicit standard. Starting with go, designed to become language independent.

Referencing [laurens video](https://x.com/poteto/status/2102050467505430555) I'll
be adapting their practices to go lang projects.

## getting started (v1: `rubric init`)

`rubric init` is implemented. It creates a Go project from tested templates, or adds
`rubric.yaml`, agent guidance, and optional tooling to an existing module. Nothing is
published yet; build it from this checkout:

```text
go install ./cmd/rubric                  # or: go build -o ./bin/rubric ./cmd/rubric
rubric init my-app --module example.com/my-app --http chi --lint --makefile
```

In a terminal it opens a wizard; with `--non-interactive`, `--format json`, or redirected
output it runs unattended. See [docs/cli-init.md](docs/cli-init.md) for every flag,
configuration precedence, conflicts, recovery, and generated test commands.

Everything below the principles is the roadmap. Items other than `rubric init` are not
implemented, and generated tooling never calls them.

## principles

- **Deterministic over prompted.** Anything that can be checked by a tool is checked
  by a tool. Skills tell agents which tool to run, not how to judge.
- **Repo local.** Skills, config, and generated tooling live in the user's repo and are
  committed. No global install state required for an agent to do the right thing.
- **Diff scoped.** Validation, path coverage, and perf checks run against code changed
  since `merge-base main`, so cost scales with the change, not the repo.
- **Prove it.** Agents back claims (coverage, performance) with measured output.

## features

- [ ] **Skills (repo local)**
  - [ ] Installed into the repo (e.g. `.claude/skills/`) with an `AGENTS.md` pointer so
        non-Claude agents find the same guidance.
  - [ ] Tracing: use OTEL tooling to find bottlenecks and report before/after deltas.
  - [ ] Issues: issue creation for different types of features.
  - [ ] Bug reporting: agents file issues themselves when they find bugs outside the
        scope of their current task, with repro steps, failing path/test, and trace or
        benchmark evidence attached. Deduped against open issues before filing.

- [ ] **Style guide**
  - [ ] [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments),
        [Peter Bourgon's Formatting and style guidance](https://peter.bourgon.org/go-in-production/#formatting-and-style),
        and the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).
  - [ ] No inline comments by agents. Doc comments on exported symbols are required
        (go convention, enforced via lint).
  - [ ] Doc comments max 2 lines, 150 chars per line. Enforced by a custom analyzer.

- [ ] **Makefile generation**
  - [ ] Detect application needs from `go.mod` imports and repo layout.
  - [ ] Write detected needs to a `rubric.yaml` the user can review and edit.
  - [ ] Generate the Makefile from `rubric.yaml` only, so output is reproducible.
  - [ ] Targets to run the application in every supported configuration.

- [ ] **Path coverage of changed code**
  - [ ] Build a control-flow graph (`golang.org/x/tools/go/ssa`) for each changed function
        and enumerate branch combinations: user types, request shapes, logic branches.
  - [ ] Run tests with coverage and mark which enumerated paths were hit.
  - [ ] Pairwise combination selection by default to avoid combinatorial explosion,
        full enumeration on demand.
  - [ ] Scaffold table-test skeletons, one row per uncovered path, for the agent to fill
        with inputs and assertions.
  - [ ] Rerun to confirm coverage. CI hard fails on any uncovered changed path.

- [ ] **Performance proof**
  - [ ] `go test -bench` on base vs change, compared with `benchstat` for statistically
        significant deltas.
  - [ ] Trace diffs: per-operation OTEL span durations across runs of the same scenario.
  - [ ] Load tests against the running app, comparing p50/p95/p99 and throughput.
  - [ ] Output is agent readable (JSON/summary files), no collector UI required.

- [ ] **CI (GitHub Actions)**
  - [ ] Generated workflow that calls `rubric validate`.
  - [ ] Linting as strict as possible.
  - [ ] LSP diagnostics down to hint/info level: complies exactly with anything the go
        toolchain thought was useful.
  - [ ] Path coverage gate and perf reports on PRs.

- [ ] **CLI**
  - [x] `rubric init`: interactive TUI to select features and bootstrap the repo.
        Uses Bubble Tea, supports new and existing Go projects, and also runs
        noninteractively. See [docs/cli-init.md](docs/cli-init.md) and the
        [v1 design spec](docs/superpowers/specs/2026-09-24-rubric-init-design.md).
  - [ ] Asks a series of questions to set up initial prompts.
  - [ ] `rubric validate`: lint, LSP conformance, path coverage.
  - [ ] `rubric make`: detect needs, update `rubric.yaml`, regenerate the Makefile.
  - [ ] `rubric paths`: list changed-code paths, coverage status, scaffold tests.
  - [ ] `rubric perf`: run benchmarks, trace diffs, load tests, emit before/after report.

- [x] **Testing (v1)**
  - [x] Automated tests cover all Rubric behavior (`go test -race ./...`).
  - [x] Generated Go code includes meaningful tests, except `main.go` files and files
        beneath `cmd/` directories. Keep entry points thin and behavior in tested packages.
  - [x] Generated tests are included even when Makefile or CI setup is disabled.
  - [x] Every supported combination is generated, built, and tested by
        `RUBRIC_ACCEPTANCE=1 go test ./internal/acceptance` (needs `TEST_DATABASE_URL`
        for an isolated PostgreSQL test database).

## open questions

- Infeasible paths: how to mark SSA paths that can't be reached (annotation, config, or
  solver)?
- Load test tool: k6, vegeta, or built in?
- Issues skill: which issue types, and GitHub Issues only?
- Bug issues: file directly, or draft for human approval first? Label/severity scheme?
- Language independence: which parts become a per-language plugin interface?
