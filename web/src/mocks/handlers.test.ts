import { describe, expect, it } from 'vitest'

// Proves the src/test/setup.ts wiring (server.listen / resetHandlers /
// close) actually intercepts, and that the two fixture tiers reach the
// wire in the shapes the contract fixes.
describe('MSW mock backend', () => {
  it('serves the contract-generated health fixture (tier a)', async () => {
    const res = await fetch('http://localhost/healthz')
    expect(res.status).toBe(200)
    expect(await res.json()).toEqual({})
  })

  it('answers unknown /api/v1 paths with FR-5 error shape (tier b)', async () => {
    const res = await fetch('http://localhost/api/v1/does-not-exist')
    expect(res.status).toBe(404)
    expect(await res.json()).toMatchObject({
      code: expect.any(String),
      message: expect.any(String),
      correlationId: expect.any(String),
    })
  })
})
