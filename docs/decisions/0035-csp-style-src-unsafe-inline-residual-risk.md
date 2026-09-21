# 0035. Content-Security-Policy style-src 'unsafe-inline' residual risk

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-10 |
| **Deciders** | Maintainer, via Phase 16 (audit 0016 #191) |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 16's security audit flagged issue **#191**:
The outer SPA document's Content-Security-Policy header (`appCSP` in
`internal/transport/http/security_headers.go`) allows
`style-src 'self' 'unsafe-inline'`, accompanied by a source comment
saying "revisit with nonces in phase 16".

`script-src 'self'` has no `'unsafe-inline'` and is fully intact; the
Vite build emits no inline scripts. However, `'unsafe-inline'` under
`style-src` theoretically permits injected styles to execute CSS-based
attacks (e.g. CSS attribute selectors for exfiltration, or UI redress/
spoofing).

## Decision

We retain `style-src 'self' 'unsafe-inline'` for the outer SPA document,
replace the inline Phase 16 TODO promise in shipped code with a citation to
this ADR, and record the residual risk evaluation.

### Rationale

1. **Client architecture:** Alexandryn is a React SPA built with Radix UI
   primitives (`@radix-ui/react-*`), Lucide icons, and Tailwind CSS.
   Radix primitives dynamically inject runtime `<style>` blocks and inline
   style attributes for portal dimensions, scroll locks, popover placement,
   and animations.
2. **Static SPA distribution:** The Go backend serves the compiled SPA
   as static files via `go:embed` / standard HTTP static serving. Implementing
   per-response CSP nonces would require dynamic server-side template
   rendering of `index.html` on every HTTP request and coordinating nonce
   passing to dynamic React/Radix style injections across chunk boundaries.
   This would fundamentally alter the zero-cost static asset architecture
   with substantial complexity and risk of runtime style-breakage.
3. **Defense-in-depth boundaries:**
   - The reader iframe (where untrusted user-supplied EPUB and document content
     renders) uses a completely isolated, strict policy without unsafe-inline
     (ADR 0024) and does not share this outer SPA policy.
   - User input rendered in the SPA interface is escaped by React's JSX virtual DOM
     model, preventing arbitrary HTML/style injection.
   - `connect-src 'self'` prevents external network exfiltration by injected styles
     or stylesheets.

## Consequences

- The outer application continues to function reliably across all Radix UI
  and dynamic styling features.
- No obsolete "phase 16 TODO" comment remains in shipped Go source.
- Residual risk: an XSS vulnerability capable of injecting raw HTML `<style>` tags
  could apply arbitrary CSS rules in the context of the outer app window.
  Mitigated by React's standard escaping, strict `script-src 'self'` (no script
  injection), and isolated sandboxed iframes for reader content.
