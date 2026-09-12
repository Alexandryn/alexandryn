import { http, HttpResponse, delay } from 'msw'
import { afterEach, describe, expect, it } from 'vitest'

import { server } from '../mocks/node'
import { deleteRequest, getJson, patchJson, postJson, putJson } from './http'

describe('http client AbortSignal forwarding', () => {
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

  it('handles 200 response with empty body without throwing JSON parse error', async () => {
    server.use(http.delete('*/api/v1/empty-200', () => new Response('', { status: 200 })))
    await expect(deleteRequest('/api/v1/empty-200')).resolves.toBeUndefined()
  })

  it('surfaces an error when a JSON endpoint returns an empty body (does not silently yield undefined)', async () => {
    server.use(
      http.get(
        '*/api/v1/broken-json',
        () => new Response('', { status: 200, headers: { 'Content-Type': 'application/json' } }),
      ),
    )
    await expect(getJson('/api/v1/broken-json')).rejects.toBeInstanceOf(Error)
  })

  it('treats an empty 205 response as a void result', async () => {
    server.use(http.post('*/api/v1/reset-content', () => new Response(null, { status: 205 })))
    await expect(postJson('/api/v1/reset-content', { x: 1 })).resolves.toBeUndefined()
  })
})

function mkAborted(): AbortSignal {
  const c = new AbortController()
  c.abort()
  return c.signal
}
