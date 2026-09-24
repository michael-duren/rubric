Never co author commites

## Skills

- Engineering skills live in `.agents/skills/`, vendored from [pstack](https://github.com/cursor/plugins/tree/main/pstack)
  by `scripts/vendor-pstack.sh`. Start with `poteto-mode` for rigorous work; subagents are in `.agents/agents/`.
- Do not edit vendored skills by hand. Change `scripts/vendor-pstack.sh` and rerun it; it updates both `.agents/`
  and the copy embedded for `rubric init --skills` (`internal/render/templates/pstack/`), and a test fails if they drift.
