// Hand-written fixture: the
// contract (api/openapi.yaml) does not define per-endpoint /api/v1 error
// responses yet, so this is hand-written — but it mirrors standard
// API error shape (code / message / correlationId)
// exactly, so replacing it with a generated fixture later is like-for-like.
// TODO: replace with contract-generated fixture

export const notFoundError = {
  code: 'not_found',
  message: 'no such endpoint',
  correlationId: 'mock-0000-0000-0000-notfound',
} as const
