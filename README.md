# go skills

## about

A repo dedicated to agentic go skills and deterministic tooling around agents.

Referencing [https://x.com/poteto/status/2102050467505430555](laurens video) I'll
be adapting their practices to go lang projects.

## features

- [ ] Deterministic tooling: script that generates makefile updates based on appication
      needs, tracing standardization for measuring performance characteristics.
- [ ] Style Guide: Referenced go lang style guides, best practices, no comments by agents.
- [ ] CI: Linting as strict as possible, LSP/Hint INFO level diagnostic: complies exactly
      with anything the go compilers thought was useful.
- [ ] Skills:
  - [ ] Tracing: use the OTEL tracing tooling to evaluate performance bottlenecks.
  - [ ] Issues: issue creation for different types of features.
- [ ] CLI:
  - [ ] Bootstraps users project with an interactive tui for selecting go skill features.
  - [ ] Users a serious of questions to setup initial prompts to get started.
  - [ ] run/test/validate: for agents to run deterministic tooling locally and in CI around validation, makefile generation, observability information, LSP conformance and more.
