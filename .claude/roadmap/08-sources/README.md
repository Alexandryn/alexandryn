# Phase 08 — Sources

*Outline — expanded to a full phase document when phase 06 closes.*

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 06 |
| **Blocks** | 09, 10 |

## Objective

The source abstraction (identity, capabilities, availability) from the
phase 02 domain, implemented against a first real provider — local folder
and OPDS — with every source response treated as hostile input.

## Scope

**In**

- Source domain interfaces and capability model
- Local folder provider; OPDS 1.2/2.0 provider
- Source configuration UI, health checks, credential storage
- Hostile-input handling for catalog responses and filenames

**Out**

- Async/background sync — phase 09. Import pipeline — phase 10.

## Exit criteria

- [ ] Source abstraction isolates the domain from provider protocol details
- [ ] Local folder and OPDS providers functional
- [ ] Source credentials never appear in logs, per Constitution §8
- [ ] Maintainer approval recorded
