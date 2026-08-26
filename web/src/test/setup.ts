import '@testing-library/jest-dom/vitest'

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
