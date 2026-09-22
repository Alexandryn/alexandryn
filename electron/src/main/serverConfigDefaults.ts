// The desktop host has no first-run screen to collect an
// OPEN_LIBRARY_USER_AGENT (unlike the self-hosting guide's .env, which
// asks an operator to set one) — the server treats it as required, with
// no built-in default, so without this the bundled server refuses to
// start at all. This gives the desktop target its own honest, traceable
// default: the project and the exact version making the request.

/** Open Library's own policy asks every client to identify itself descriptively. */
export function defaultOpenLibraryUserAgent(version: string): string {
  return `Alexandryn-Desktop/${version} (+https://github.com/Alexandryn/alexandryn)`
}
