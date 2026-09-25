import { act, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { WorkGrid, WINDOW_SIZE } from './WorkGrid'
import type { WorkSummary } from '../../data/library'

// A minimal IntersectionObserver that records observed targets and lets a
// test drive their intersection.
class FakeIO {
  static instances: FakeIO[] = []
  targets = new Set<Element>()
  cb: IntersectionObserverCallback
  constructor(cb: IntersectionObserverCallback) {
    this.cb = cb
    FakeIO.instances.push(this)
  }
  observe(el: Element) {
    this.targets.add(el)
  }
  unobserve(el: Element) {
    this.targets.delete(el)
  }
  disconnect() {
    this.targets.clear()
  }
  fire(el: Element) {
    this.cb(
      [{ target: el, isIntersecting: true } as IntersectionObserverEntry],
      this as unknown as IntersectionObserver,
    )
  }
}

function makeMockWorks(count: number): WorkSummary[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `work-${i + 1}`,
    title: `Book Title ${i + 1}`,
    subtitle: i % 2 === 0 ? `Subtitle ${i + 1}` : '',
    authors: [`Author ${i + 1}`],
    isOwned: i % 3 !== 0,
    collections: i % 2 === 0 ? [{ id: 'coll-1', name: 'Favorites' }] : [],
    addedAt: '2026-01-15T10:00:00Z',
  }))
}

describe('WorkGrid', () => {
  it('renders grid view correctly for <= 100 items without placeholders', () => {
    const works = makeMockWorks(10)
    render(
      <MemoryRouter>
        <WorkGrid works={works} view="grid" />
      </MemoryRouter>,
    )

    expect(screen.getAllByText('Book Title 1').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Book Title 10').length).toBeGreaterThan(0)
    expect(screen.queryAllByTestId('virtual-cover-placeholder')).toHaveLength(0)
  })

  it('renders list view with subtitles and collection badges', () => {
    const works = makeMockWorks(5)
    render(
      <MemoryRouter>
        <WorkGrid works={works} view="list" />
      </MemoryRouter>,
    )

    expect(screen.getAllByText('Book Title 1').length).toBeGreaterThan(0)
    expect(screen.getByText('Subtitle 1')).toBeInTheDocument()
    expect(screen.getAllByText('Favorites').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Wanted').length).toBeGreaterThan(0)
  })

  it('virtualizes when items exceed 100 threshold', () => {
    const works = makeMockWorks(120)
    render(
      <MemoryRouter>
        <WorkGrid works={works} view="grid" />
      </MemoryRouter>,
    )

    // Above threshold (120 > 100), items beyond initial visible count (40) render placeholders
    const placeholders = screen.getAllByTestId('virtual-cover-placeholder')
    expect(placeholders.length).toBe(120 - 40)
  })

  it('skips mounting link and metadata for items outside the sliding window in grid view', () => {
    const works = makeMockWorks(120)
    const { container } = render(
      <MemoryRouter>
        <WorkGrid works={works} view="grid" />
      </MemoryRouter>,
    )

    // Items within window are fully mounted (links and titles present)
    expect(container.querySelector('a[href="/book/work-1"]')).not.toBeNull()
    expect(container.querySelector('a[href="/book/work-40"]')).not.toBeNull()

    // Items outside window do not mount link or text
    expect(container.querySelector('a[href="/book/work-41"]')).toBeNull()
    expect(container.querySelector('a[href="/book/work-120"]')).toBeNull()
    expect(screen.queryByText('Book Title 120')).toBeNull()
  })

  it('skips mounting link and metadata for items outside the sliding window in list view', () => {
    const works = makeMockWorks(120)
    const { container } = render(
      <MemoryRouter>
        <WorkGrid works={works} view="list" />
      </MemoryRouter>,
    )

    // Items within window are fully mounted
    expect(container.querySelector('a[href="/book/work-1"]')).not.toBeNull()
    expect(container.querySelector('a[href="/book/work-40"]')).not.toBeNull()

    // Items outside window do not mount link or text
    expect(container.querySelector('a[href="/book/work-41"]')).toBeNull()
    expect(container.querySelector('a[href="/book/work-120"]')).toBeNull()
    expect(screen.queryByText('Book Title 120')).toBeNull()
  })

  describe('sliding window', () => {
    afterEach(() => {
      FakeIO.instances = []
      vi.unstubAllGlobals()
    })

    it('keeps the mounted-cover count bounded and unmounts covers scrolled past', () => {
      vi.stubGlobal('IntersectionObserver', FakeIO)
      const { container } = render(
        <MemoryRouter>
          <WorkGrid works={makeMockWorks(300)} view="grid" />
        </MemoryRouter>,
      )

      const mountedCovers = () => 300 - screen.getAllByTestId('virtual-cover-placeholder').length
      const fireBottom = () => {
        const io = FakeIO.instances.at(-1)
        const bottom = container.querySelector('[data-sentinel="bottom"]')
        if (io && bottom) act(() => io.fire(bottom))
      }

      expect(mountedCovers()).toBe(WINDOW_SIZE)
      expect(container.querySelector('[data-sentinel="top"]')).toBeNull() // at the start

      // Scroll the window all the way to the bottom.
      for (let i = 0; i < 40; i++) fireBottom()

      // Still bounded — a "grow only" implementation would have mounted all 300.
      expect(mountedCovers()).toBeLessThanOrEqual(WINDOW_SIZE)
      // The window moved: work 1's cover is unmounted, and a top sentinel
      // now exists to slide back up.
      const firstItem = container.querySelector('.grid')?.firstElementChild
      expect(firstItem?.querySelector('[data-testid="virtual-cover-placeholder"]')).not.toBeNull()
      expect(container.querySelector('[data-sentinel="top"]')).not.toBeNull()
    })
  })
})
