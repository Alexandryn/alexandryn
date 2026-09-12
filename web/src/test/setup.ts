import '@testing-library/jest-dom/vitest'
import { afterAll, afterEach, beforeAll } from 'vitest'
import { server } from '../mocks/node'

// jsdom has no layout engine and doesn't implement ResizeObserver — Radix's
// Slider (and any future layout-measuring primitive) needs a stub present,
// not real measurement, to mount in tests at all.
if (typeof globalThis.ResizeObserver === 'undefined') {
  class ResizeObserverStub {
    observe(): void {}
    unobserve(): void {}
    disconnect(): void {}
  }
  globalThis.ResizeObserver = ResizeObserverStub as unknown as typeof ResizeObserver
}

// MSW intercepts every test's network calls. `error` on an unhandled request
// is deliberate — a component reaching an endpoint no fixture covers is a
// test bug, not something to let through to a real socket.
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
afterEach(() => server.resetHandlers())
afterAll(() => server.close())
