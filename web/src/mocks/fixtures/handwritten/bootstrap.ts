// Hand-written fixture (frontend-shell-and-routing.md FR-6 tier b): there
// is no bootstrap/capability endpoint in the contract yet — phase 12
// introduces the real one. Until then this returns every capability
// granted unconditionally, matching architecture-frontend.md FR-3's
// current-state rule (loopback-only, therefore trusted).
// TODO(phase-06): replace with contract-generated fixture

export const bootstrapFixture = {
  capabilities: {
    sources: true,
    import: true,
    settings: true,
    system: true,
  },
} as const
