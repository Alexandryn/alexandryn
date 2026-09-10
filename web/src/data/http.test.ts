import { http, HttpResponse, delay } from 'msw'
import { afterEach, describe, expect, it } from 'vitest'

import { server } from '../mocks/node'
import { deleteRequest, getJson, patchJson, postJson, putJson } from './http'

describe('http client AbortSignal forwarding (audit 0016 #166)', () => {
  afterEach(() => localStorage.clear())

  it('aborts an in-flight GET when the signal fires', async () => {
    server.use(
      http.get('*/api/v1/slow', async () => {
        await delay(1000)
        return HttpResponse.json({ ok: true })
      }),
    )

    const controller = new AbortController()
    const pending = getJson('/api/v1/slow', {}, { signal: controller.signal })
    controller.abort()

    await expect(pending).rejects.toMatchObject({ name: 'AbortError' })
  })

  it.each([
    ['POST', () => postJson('/api/v1/slow', { x: 1 }, {}, { signal: mkAborted() })],
    ['PUT', () => putJson('/api/v1/slow', { x: 1 }, {}, { signal: mkAborted() })],
    ['PATCH', () => patchJson('/api/v1/slow', { x: 1 }, {}, { signal: mkAborted() })],
    ['DELETE', () => deleteRequest('/api/v1/slow', {}, { signal: mkAborted() })],
  ])('forwards the signal on %s', async (_method, call) => {
    server.use(
      http.all('*/api/v1/slow', async () => {
        await delay(1000)
        return HttpResponse.json({ ok: true })
      }),
    )
    await expect(call()).rejects.toMatchObject({ name: 'AbortError' })
  })

  it('completes normally when no signal is given', async () => {
    server.use(http.get('*/api/v1/fine', () => HttpResponse.json({ value: 42 })))
    await expect(getJson<{ value: number }>('/api/v1/fine')).resolves.toEqual({ value: 42 })
  })

  it('handles 200 response with empty body without throwing JSON parse error (audit 0016 #230)', async () => {
    server.use(http.delete('*/api/v1/empty-200', () => new Response('', { status: 200 })))
    await expect(deleteRequest('/api/v1/empty-200')).resolves.toBeUndefined()
  })
})

function mkAborted(): AbortSignal {
  const c = new AbortController()
  c.abort()
  return c.signal
}
