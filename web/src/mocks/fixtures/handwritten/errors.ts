// Hand-written fixture (frontend-shell-and-routing.md FR-6 tier b): the
// contract (api/openapi.yaml) does not define per-endpoint /api/v1 error
// responses yet, so this is hand-written — but it mirrors
// architecture-contracts.md FR-5's shape (code / message / correlationId)
// exactly, so replacing it with a generated fixture later is like-for-like.
// TODO(phase-06): replace with contract-generated fixture

export const notFoundError = {
  code: 'not_found',
  message: 'no such endpoint',
  correlationId: 'mock-0000-0000-0000-notfound',
} as const
