# Security audit: <feature, phase, or component>

| | |
|---|---|
| **Scope** | |
| **Auditor** | |
| **Date** | YYYY-MM-DD |
| **Commit** | |
| **Verdict** | Clear / Findings open / Blocked |

## Scope and method

What was examined, and how — code reading, dependency review, manual probing,
fuzzing, automated scanning. Say which, so a later reader knows the depth.

## Trust boundaries examined

| Boundary | Untrusted side | Assumption being made |
|---|---|---|
| | | |

## Adversarial questions asked

Work through these explicitly. Constitution §10.

- What does a malicious **user** of this instance achieve here?
- What does a malicious **source** achieve — in its metadata, its redirects,
  its filenames, its file contents?
- What does a device on the **local network** achieve without credentials?
- What happens when input is malformed, empty, enormous, duplicated, slow, or
  never arrives?
- What is logged that shouldn't be?
- What happens if two of these run at once?

## Findings

| ID | Severity | Title | Status |
|---|---|---|---|
| A-NN-01 | | | Open / Fixed / Accepted / Won't fix |

### A-NN-01 — <title>

**Severity:** Critical / High / Medium / Low / Informational

**Component:** `path/to/thing`

**Description** — what the flaw is, mechanically.

**Impact** — what an attacker gains. Be concrete. If the honest answer is
"nothing much on its own", say that and rate it accordingly.

**Preconditions** — what the attacker needs first.

**Reproduction** — steps or a proof of concept.

**Recommendation** — the specific fix, not a category of fix.

**Resolution** — filled in when addressed, with the commit.

---

## Severity guide

Rate the actual impact in this system, not the textbook worst case for the
class of bug. Inflated ratings train people to ignore ratings.

| | |
|---|---|
| **Critical** | Remote code execution, or unauthenticated access to the library from the network |
| **High** | Authentication bypass, arbitrary file read/write, renderer sandbox escape, credential leakage |
| **Medium** | Requires user interaction or an unusual configuration; limited data exposure; persistent DoS |
| **Low** | Narrow impact, high preconditions, or defence-in-depth that's missing |
| **Informational** | No impact today, but it makes a future mistake more likely |

## What was not examined

Honest gaps in coverage.
