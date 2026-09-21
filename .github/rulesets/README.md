# Repository rulesets

The rules every Alexandryn repository uses, kept here so they can be applied
again. They describe an open-source setup: anyone can fork and open a pull
request, and only the maintainer can merge one.

- `main.json` protects the default branch. Nobody pushes to it directly: every
  change is a pull request, merged by rebase, with linear history, no force
  pushes and no deletion. A pull request needs one approving review, and only
  people with write access count, which is the maintainer alone. The repository
  admin role may bypass the review requirement, but only by merging a pull
  request, not by pushing.
- `tags.json` protects `v*` release tags. Only the admin role can create one,
  and no one can move or delete one, because a release tag is what the release
  workflow publishes from.

Apply them to a repository with:

```sh
gh api -X POST repos/Alexandryn/<repo>/rulesets --input .github/rulesets/main.json
gh api -X POST repos/Alexandryn/<repo>/rulesets --input .github/rulesets/tags.json
```

Rulesets need a public repository or a paid GitHub plan. On the free plan a
private repository refuses them, so apply these when a repository goes public.
Status checks are not required yet: add a `required_status_checks` rule to
`main.json` once CI can run.

Also set, for each public repository: merge by rebase only, delete branches on
merge, default workflow permissions read-only, approval required for workflows
from outside contributors, and secret scanning with push protection.
