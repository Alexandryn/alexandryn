// GENERATED FILE — do not hand-edit.
// Run `npm run mocks:gen-fixtures` (web/scripts/gen-fixtures.ts) to
// regenerate from api/openapi.yaml. frontend-shell-and-routing.md FR-6
// tier (a): fixtures for contract-covered endpoints are generated, never
// hand-written. Hand-written fixtures for endpoints the contract does not
// cover yet live in ../handwritten/ and carry a TODO(phase-06) marker.

export const generatedFixtures = {
  getHealthz: {
    '200': {},
  },
  getReadyz: {
    '200': {},
    '503': {
      code: 'unavailable',
      message: 'not yet started',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
} as const
