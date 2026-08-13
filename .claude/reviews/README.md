# Reviews

Records of specification and change reviews. Start from
[`../templates/review.md`](../templates/review.md).

A review recorded here is a gate, not a comment thread. It exists so that
"this was approved" is a checkable fact months later, and so the reasoning
behind an approval survives the pull request UI.

## What gets a recorded review

- Every specification, before it can reach `APPROVED`
- Every architecture decision record, before it reaches `Accepted`
- Changes that alter a trust boundary, a domain boundary, or a public contract

Ordinary pull requests are reviewed in GitHub and don't need a file here.

## Verdicts

| Verdict | Means |
|---|---|
| Approved | Proceed. |
| Approved with changes | Proceed once the listed changes are made; no second review needed. |
| Needs rework | Substantial gaps. Revise and resubmit for a full review. |
| Rejected | The approach is wrong, not just incomplete. The record explains why. |

## Reviewing well

Look for contradictions before style. A spec that disagrees with the
constitution, with an ADR, or with itself is a more expensive problem than an
awkward sentence.

Ask what two different engineers would build from the same document. If the
answers differ, the spec is ambiguous regardless of how well written it is.

Mark the dimension checklist honestly. An unchecked box is useful information.
A falsely checked one is worse than no checklist at all, because it stops the
next person from looking.

## Naming

`NNNN-<subject>.md`, sequential.

## Index

| Review | Subject | Date | Verdict |
|---|---|---|---|
| — | | | |
