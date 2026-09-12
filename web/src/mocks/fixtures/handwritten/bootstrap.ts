// Hand-written fixture: there
// is no bootstrap/capability endpoint in the contract yet.
// Until then this returns every capability
// granted unconditionally, matching the loopback-only trusted rule.
// TODO: replace with contract-generated fixture

export const bootstrapFixture = {
  capabilities: {
    sources: true,
    import: true,
    settings: true,
    system: true,
    network: true,
  },
} as const
