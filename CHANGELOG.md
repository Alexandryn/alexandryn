# Changelog

All notable changes to Alexandryn are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/).

## [1.0.2] - 2026-09-22

### Fixed

- The container image now creates the credential-key directory owned by the
  application user. On real Docker, a new `app-data` volume mounted over a
  directory the image did not have was created root-owned, so the server could
  not write its credential key (`permission denied`) and crash-looped. The
  bundled stack was first verified under Podman, where a new volume behaves
  differently, which is how this shipped in 1.0.0 and 1.0.1 without being
  caught. If you deployed either of those versions with the bundled database,
  see the self-hosting guide for how to fix an existing `app-data` volume.

## [1.0.1] - 2026-09-21

### Fixed

- The Windows and macOS installers can now be built. The release packaging
  step looked up the application binary by a name that electron-builder only
  provides on Linux, so the 1.0.0 release run built a Linux installer and
  failed on the other two platforms. Windows, macOS and Linux installers, and
  their checksums, are attached to this release.

No changes to the application, the API contract or the container image
behaviour. The API contract remains version 1.0.0.

## [1.0.0] - 2026-09-21

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
- A Docker/Compose deployment target for running Alexandryn on a server
  or NAS instead of (or alongside) the desktop app, with a bundled
  PostgreSQL option for a single-command start.
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
- The desktop app's packaged binary has Electron's own security fuses
  set: it cannot be launched as a raw Node process, cannot receive
  debugger or `NODE_OPTIONS` arguments, and only loads its own signed
  application archive.
- The Docker deployment target runs as a non-root user, publishes no
  network port by default, refuses to start with a placeholder database
  password, and keeps your encryption key and stored source credentials
  on a persistent volume that survives an image update or container
  restart.
- The desktop installers are not yet code-signed (macOS notarization,
  Windows Authenticode). Your operating system will warn you about this
  on first launch; this is expected until signing is set up in a future
  release.
