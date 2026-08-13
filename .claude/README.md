# `.claude/` — the project's engineering memory

This directory is a first-class part of Alexandryn, not scaffolding around it.
It holds the reasoning: why the system is shaped the way it is, what was
considered and rejected, what is known to be unfinished.

Code says what the system does. This says why, and what it must never do.

## Layout

| Path | Holds | Written when |
|---|---|---|
| `constitution.md` | The rules that don't bend | Once; amended by PR |
| `roadmap/` | Phases in dependency order, with exit criteria | Before work starts |
| `specs/` | Feature specifications and their status | Before implementation |
| `decisions/` | ADRs, including open questions | When a choice has lasting consequences |
| `test-plans/` | What gets tested, at which layer, and why there | With the spec |
| `reviews/` | Spec and change review records | At the review gate |
| `audits/` | Adversarial reviews and findings | At the audit gate |
| `skills/general/` | Reusable engineering practice guides | As practices stabilise |
| `skills/purpose/` | Alexandryn-specific practice guides | As the domain stabilises |
| `templates/` | The shape every document above takes | Once; refined in use |

## Rules for this directory

**Use the templates.** Consistent structure is what makes these documents
searchable and comparable. If a template is wrong, fix the template.

**Status fields are honest.** A spec marked `APPROVED` means someone reviewed
it. A phase marked `Closed` means its exit criteria are actually met. These
labels are load-bearing — automated contributors gate on them.

**Nothing is deleted.** Superseded documents get a status change and a pointer
to what replaced them. The wrong turns are part of the record; a future
contributor needs to know an option was tried, not just that it isn't here.

**Detail decreases with distance.** Near phases are specified thoroughly.
Distant phases are outlines, because writing detailed requirements for phase 14
before phase 02 exists produces confident fiction. Outlines are marked as such
and expanded when the phase is approached.

**No secrets, no personal data.** This directory will be public.

## Reading order for a newcomer

1. `constitution.md`
2. `roadmap/README.md`
3. The lowest-numbered open phase
4. `decisions/` — skim titles, read the ones that surprise you
