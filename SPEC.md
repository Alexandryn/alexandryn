# Spec pointer

This file exists so tooling that looks for `SPEC.md` at the repository root
(for example the agent-skills pack's `/spec` and `/build auto` commands)
finds this note instead of finding nothing. It is not itself a specification.

Alexandryn's real spec corpus lives in [`.claude/specs/`](.claude/specs/),
one file per feature or subsystem, indexed with status in
[`.claude/specs/README.md`](.claude/specs/README.md). Architecture decisions
are recorded separately in [`.claude/decisions/`](.claude/decisions/). Start
with [`CLAUDE.md`](CLAUDE.md) and
[`.claude/constitution.md`](.claude/constitution.md) before reading either.
