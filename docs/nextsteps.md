# Next steps

Working notes on where Rubric stands and what comes next, centered on feature maps.
Nothing here is decided; the open questions at the end are for the maintainer.

## Where we are (2026-09-24)

Shipped on `main`:

- `rubric init` v1 (#1): Bubble Tea wizard and unattended mode, new and existing Go projects,
  reviewed file plans, ownership manifest, conflict handling, rollback, project-aware `AGENTS.md`,
  optional Makefile, lint (golangci-lint plus a comment analyzer), GitHub Actions, and agent skills.
- Web UI option (#2): htmx, Alpine.js, and templ, with optional Playwright browser tests.
- Every generated project gets an executable under `cmd/` unless `--starter module` is chosen, and the
  wizard is styled (#6).
- Repository hygiene: branch protection script and ruleset (#3), path-filtered CI behind one required
  gate (#4, #7), squash-only merges, branches deleted after merge.

In review:

- #8 vendors [pstack](https://github.com/cursor/plugins/tree/main/pstack) into `.agents/skills/` here and
  into projects generated with `--skills`. pstack's `create-verification-skill` and
  `maintain-verification-skill` are the starting point for feature maps.

Verification today: `go test -race ./...`, golangci-lint, and an acceptance suite that generates every
supported configuration and runs its build, tests, coverage check, and a rerun no-op check.

## Why feature maps instead of every code path

The original roadmap item "path coverage of changed code" enumerates control-flow paths. That does not
scale: paths multiply with every branch, and most combinations are impossible or uninteresting.

A feature map changes the unit of coverage from paths to behaviors the product promises. It answers:
does every scenario of every feature have a test (or a recorded manual verification) that actually
exercises that feature's code?

### Exhaustive versus pairwise combinations

Rubric's own acceptance suite is the exhaustive case. The catalog has a handful of independent choices
(HTTP, database and access, CLI, TUI, configuration, web and e2e). Multiplying the valid choices gives
420 feature combinations; ten of them have no executable and are generated with both starters, so the
suite builds and tests 430 projects. That is affordable only because the catalog is small and each
project builds in seconds. One more three-way option would triple it.

Pairwise testing picks a much smaller set in which every pair of choices appears together at least once.
Most interaction bugs involve two features, so it catches most of them cheaply. That is the practical
approach for real applications, whose feature combinations explode.

## What a feature map is

An explicit, versioned description of the application's user-facing features. Each feature links:

1. Where it lives: entry points a user reaches (HTTP route, CLI command, TUI screen) and the code that
   implements it.
2. What it promises: short sub-feature IDs, such as success, invalid input, dependency failure, and
   cancellation, matching the scenarios the spec already asks tests to cover.
3. What proves it: tests, or a scripted live drive with captured evidence.

### The pstack format

pstack stores the map as markdown inside a project-local verification skill, readable and editable by
people and agents:

- `.agents/skills/verify-<app>/SKILL.md` explains launch, doctor (health check), drive (the harness),
  evidence, and cleanup for the real app.
- `.agents/skills/verify-<app>/features/README.md` is the index, with baseline preconditions, driving
  conventions, and proof rules.
- One file per feature, with exactly four sections: `Sub-features`, `How to get to it (user POV)`,
  `Driving it with <harness>`, and `Gotchas`.

`create-verification-skill` generates this from the repository and proves it by driving one feature.
`maintain-verification-skill` keeps it honest: agents read the source per feature and drive every
feature live, then ship at most one pull request of corrections.

### Notes from Lauren Tan's talk

- Controlled glass: teach the agent how to run the application and trace it.
- Observability around agents, locally: compare agent efficiency by the skills they use, verification,
  code quality, coverage, and so on.
- Feature map: how the application works, its features, and how a user reaches each one. Stored in a
  skill as markdown, but controlled through a CLI so drift can be detected.
- The codebase is the best evidence of how agents should behave; they imitate what is already there.
  Invest in the codebase, static analysis, review rules (bugbot), skills, and a style guide.
- Your codebase is memory.

## Proposed direction: `rubric features`

pstack's maintenance loop detects drift with agents. The gap is a mechanical, CLI-owned check that runs
in CI without an agent. A sketch, not a spec:

- Scaffold: create the verify skill and seed feature files from what Rubric already knows (generated
  HTTP routes, CLI commands, TUI), in pstack's format.
- Edit: add, rename, or retire features and sub-features without hand-editing structure.
- Structure check: every feature file has the four sections; the index matches the files; sub-feature IDs
  are unique.
- Code drift: extract real entry points from source (Chi and `net/http` routes, Cobra and `flag`
  commands) and compare them with the map. Report unmapped entry points and mapped ones that no longer
  exist.
- Test drift: each sub-feature maps to tests that exist, pass, and execute that feature's code, measured
  with per-test coverage profiles (and `go build -cover` with `GOCOVERDIR` for end-to-end runs of the
  built binary).
- Diff scope: map changed files to the features that own them, so agents and CI verify only those.
- Agents: generated `AGENTS.md` and skills tell agents to read the map before changing a feature, update
  it in the same change, and run the check.

This likely replaces the roadmap's `rubric paths` and overlaps `rubric validate`.

## Open questions

1. Who writes the map: people, Rubric (inferred from routes and commands, then corrected), or agents via
   `create-verification-skill`? Any is acceptable; drift detection is required.
2. Where does the map live: inside the pstack-style `verify-<app>` skill, or a Rubric-owned location
   that the skill references? How much of the file format does the CLI own versus free prose?
3. How are sub-features linked to tests: a table in the feature file, test naming conventions, or a
   separate index? Rubric's style policy forbids explanatory comments, so comment annotations are out
   unless they become a documented directive.
4. How is code linked to features: declared packages or globs, call-graph reachability from entry points,
   or observed coverage? Shared helpers belong to many features; how is that reported?
5. Should drift fail CI or only report? Per feature, per sub-feature, or only for features touched by the
   diff?
6. Granularity: one feature per route or command, or per user-facing capability spanning several entry
   points?
7. Scope: generated projects first, any Go repository, or Rubric itself as the first user?
8. Live verification: should `rubric features` drive the app (the pstack harness), or only check static
   and test evidence and leave driving to agents?
9. Pairwise configuration testing: worth offering to generated projects that have feature flags or
   configuration matrices?
10. Agent observability (from the talk): what should be measured locally to compare agents and skills
    (verification performed, coverage, review findings), and where is it stored?
11. pstack upkeep: how often to re-vendor, and whether to adapt more Cursor-specific runtime references
    (transcript paths, the model rule `setup-pstack` writes).

## Known deferred items

Minor findings accepted during reviews, not yet fixed:

- Updating an existing file resets its permissions to the rendered mode.
- Detection records per-file import evidence, so ordinary code edits change `rubric.yaml` evidence.
- New executables in an existing project are not added once `entry_points` is saved.
- Entry-point detection uses the host's build constraints.
- `--format json` emits no JSON on flag parse errors.
- In the wizard, skipping one conflicting tooling file clears other tooling decisions.
- An apply that completes during cancellation still exits 130.
- If an existing project defines its own `test-e2e` command, guidance can mention `e2e/` and
  `setup-e2e`.
- Behind a reverse proxy that rewrites `Host`, the generated web UI's cross-origin check can reject
  same-site requests.
- The generated Bubble Tea application is unstyled.
- Retargeting a pull request's base branch does not rerun CI.
