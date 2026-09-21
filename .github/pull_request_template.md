<!--
Keep this honest. A checked box that isn't true costs more than an unchecked
one, because it stops the reviewer from looking.
-->

## What changed

<!-- One paragraph. What does this do, and why now? -->

## Related

- Spec: <!-- docs/specs/… or "none — see below" -->
- Phase: <!-- docs/roadmap/NN-… -->
- Issue: <!-- #123 -->

If there is no spec, say why this change doesn't need one. Typos, dependency
bumps and build fixes don't. Behaviour changes do.

## How it was verified

<!--
Name the tests. "Tests pass" is not a verification statement — which tests
cover the new behaviour, and what would have failed before this change?
-->

## Definition of done

- [ ] Behaviour is covered by tests that fail without this change
- [ ] Error and empty states are handled, not just the happy path
- [ ] Keyboard and screen-reader behaviour considered for any UI
- [ ] Untrusted input (source responses, files, user text) is validated
- [ ] No secrets, tokens or personal paths in the diff
- [ ] Docs updated if behaviour or setup changed

## Risks

<!--
What could this break, and what would the first symptom be? Write "none I can
see" only if you actually looked.
-->
