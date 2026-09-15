# Changelog

All notable changes to Alexandryn are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/).

## [1.0.0] - Unreleased

Initial public release.

### Added

- Self-hosted digital library: an Electron desktop application that hosts
  your library and serves it to a browser on your own machine or, when you
  choose to enable it, to other devices on your home network.
- Library browsing, searching, filtering, sorting, and collections.
- Automatic metadata lookup and normalization against Open Library for
  imported works and editions.
- Source support for local folders and OPDS 1.2/2.0 catalogs, including
  credential storage and periodic health checks.
- An import pipeline that discovers files in a configured source, matches
  them against metadata, and asks for confirmation before adding them to
  your library.
- An in-browser EPUB reader with typography preferences, reading-position
  tracking, bookmarks, highlights, and a table of contents.
- Accounts with password-based sign-in, optional TOTP multi-factor
  authentication, and session management with brute-force rate limiting.
- Multiple libraries with role-based access control, for households that
  want separate collections or separate permission levels.
- Device pairing for controlled network access beyond your own machine:
  off by default, and only reachable elsewhere once you explicitly enable
  it with the required authentication and TLS in place.
- Reading progress, bookmarks, and highlights synchronized across your
  paired devices, with deterministic conflict resolution when two devices
  update the same book while offline.
- A settings screen listing your paired devices, with the ability to
  revoke any of them.
- Health and diagnostics endpoints for operators running their own
  instance.
- A keyboard-operable, screen-reader-tested interface conforming to WCAG
  2.1 AA, verified across desktop and mobile viewports.

### Security

- Every non-health API route requires authentication. Passwords are
  hashed with Argon2id; no plaintext password is ever logged or stored.
- Session tokens are purpose-typed: a token issued for one purpose (for
  example, multi-factor verification) is rejected if it's presented on a
  route that expects a different kind of token.
- Your reading data — progress, bookmarks, highlights, preferences — is
  always scoped to your account and your library. A request for another
  user's or another library's data is refused, not merely hidden from the
  interface.
- Network exposure beyond your own machine is off by default. Enabling it
  requires authentication and TLS under one of two verified configurations
  (an upstream reverse proxy already terminating TLS, or a certificate
  Alexandryn manages itself); there is no supported way to expose your
  library without both.
- Every file from a configured source, and every response from an
  external service, is treated as untrusted input: size-limited,
  shape-checked, and time-limited before anything is done with it.
- EPUB content is rendered in a sandboxed context so that a malicious book
  file cannot reach the rest of the application.
- A whole-application adversarial security review covered every trust
  boundary in the system before this release; findings are tracked
  publicly in this repository's issue tracker.

<!--
This section will be extended before the v1.0.0 tag is cut, once the
Electron installer hardening and Docker image hardening steps of the
release phase land, so those checks are reflected here alongside the
application-level work above rather than asserted ahead of the work
actually happening.
-->
