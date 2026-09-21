# Security policy

## Reporting a vulnerability

Please report privately, not in a public issue.

Use GitHub's private reporting:
[**Report a vulnerability**](https://github.com/Alexandryn/alexandryn/security/advisories/new)

What helps:

- what an attacker gains, concretely
- the steps to reproduce it, including any configuration that matters
- the version, commit, or branch you tested
- whether it needs local access, a compromised source, or nothing at all

You'll get an acknowledgement within a few days. This is a small project, so
please read that as a good-faith target rather than a contractual one. You'll
be credited in the advisory unless you'd rather not be.

Please give a reasonable window before disclosing publicly. If a report goes
unanswered for two weeks, treat that as the point to escalate or publish —
silence shouldn't trap a finding indefinitely.

## Scope

Alexandryn is a self-hosted application that intentionally serves a web
interface on a user's local network. That makes some things vulnerabilities and
some things design.

**In scope**

- Remote code execution, path traversal, or arbitrary file read/write on the
  host
- Escaping the Electron renderer sandbox, or abusing the preload/IPC boundary
  to reach privileged operations
- Authentication bypass, session fixation or theft, privilege escalation
  between accounts or devices
- Server-side request forgery through a configured source, including redirect
  and DNS-rebinding tricks
- Malicious source responses or book files causing memory exhaustion, zip
  bombs, or crashes that survive a restart
- Stored or reflected XSS, including through book metadata, filenames, or EPUB
  content rendered in the reader
- CSRF against state-changing endpoints
- Leakage of source credentials, session tokens, or reading history into logs,
  error messages, or network responses
- Anything that lets a device on the LAN reach the library without a credential

**Not in scope**

- Attacks requiring an already-compromised host account. If someone is running
  code as you, the library is the least of it.
- A user deliberately configuring a source they know to be malicious *and*
  approving its content, where the impact stays inside that source's data.
- Denial of service by an authenticated, legitimate user against their own
  instance.
- Missing hardening headers with no demonstrated impact — worth a regular
  issue, not an advisory.
- Findings from automated scanners with no working proof of concept.
- Vulnerabilities in Open Library or a third-party source server. Report those
  to their maintainers; if Alexandryn handles the response unsafely, that part
  is ours.

## Supported versions

| Version | Supported |
|---|---|
| Latest release (`main`) | :white_check_mark: |

## Our commitments

- Security work happens continuously, not at the end. Substantial features undergo
  adversarial security review before it is closed.
- The host binds to loopback by default and will not serve the network without
  authentication.
- We don't log what you read.
- If a vulnerability affects released users, we publish an advisory — including
  when it's embarrassing.
