// jsdom has no matchMedia. This installs a controllable mock and returns
// a setter that flips the match state and notifies subscribers, so a
// viewport-reflow can be simulated in a test (Checkpoint P4-E).

export interface MatchMediaControl {
  set(matches: boolean): void
  restore(): void
}

export function mockMatchMedia(initialMatches: boolean): MatchMediaControl {
  const original = window.matchMedia as typeof window.matchMedia | undefined
  const listeners = new Set<(event: MediaQueryListEvent) => void>()
  let matches = initialMatches

  const mql = {
    get matches() {
      return matches
    },
    media: '',
    onchange: null,
    addEventListener: (_type: string, cb: (event: MediaQueryListEvent) => void) =>
      listeners.add(cb),
    removeEventListener: (_type: string, cb: (event: MediaQueryListEvent) => void) =>
      listeners.delete(cb),
    addListener: (cb: (event: MediaQueryListEvent) => void) => listeners.add(cb),
    removeListener: (cb: (event: MediaQueryListEvent) => void) => listeners.delete(cb),
    dispatchEvent: () => true,
  }

  window.matchMedia = ((query: string) => {
    mql.media = query
    return mql as unknown as MediaQueryList
  }) as typeof window.matchMedia

  return {
    set(next: boolean) {
      matches = next
      for (const cb of listeners) cb({ matches } as MediaQueryListEvent)
    },
    restore() {
      if (original) window.matchMedia = original
      else delete (window as Partial<Window>).matchMedia
    },
  }
}
