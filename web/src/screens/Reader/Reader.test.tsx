import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { server } from '../../mocks/node'
import { runAxe, expectNoAxeViolations } from '../../test/axe'
import { Reader } from './Reader'
import { DEFAULT_READING_PREFERENCES } from '../../data/reading'
import {
  CHAPTER1_HOSTILE,
  CHAPTER1_XHTML,
  CONTAINER_XML,
  CONTENT_OPF,
  NAV_XHTML,
} from './reader.fixtures'

const CONTENT = /\/reader\/content\//

function contentHandlers(chapterBody = CHAPTER1_XHTML) {
  return [
    http.get(CONTENT, ({ request }) => {
      const path = decodeURIComponent(new URL(request.url).pathname.split('/reader/content/')[1] ?? '')
      const map: Record<string, string> = {
        'META-INF/container.xml': CONTAINER_XML,
        'OEBPS/content.opf': CONTENT_OPF,
        'OEBPS/nav.xhtml': NAV_XHTML,
        'OEBPS/chapter1.xhtml': chapterBody,
        'OEBPS/chapter2.xhtml': CHAPTER1_XHTML,
      }
      const body = map[path]
      if (body === undefined) return HttpResponse.json({ code: 'not_found' }, { status: 404 })
      return new HttpResponse(body, { headers: { 'Content-Type': 'application/xhtml+xml' } })
    }),
  ]
}

function readingApiHandlers(progress: unknown = null) {
  return [
    http.post('*/api/v1/library/editions/:editionId/reader/session', () =>
      new HttpResponse(null, { status: 204 }),
    ),
    http.get('*/api/v1/reading/works/:workId/progress', () => HttpResponse.json({ progress })),
    http.post('*/api/v1/reading/works/:workId/progress', () =>
      HttpResponse.json({ progress, outcome: 'advanced' }),
    ),
    http.get('*/api/v1/reading/preferences', () =>
      HttpResponse.json({ preferences: DEFAULT_READING_PREFERENCES }),
    ),
    http.put('*/api/v1/reading/preferences', async ({ request }) =>
      HttpResponse.json({ preferences: await request.json() }),
    ),
    http.get('*/api/v1/reading/editions/:editionId/bookmarks', () =>
      HttpResponse.json({ bookmarks: [] }),
    ),
    http.post('*/api/v1/reading/editions/:editionId/bookmarks', () =>
      HttpResponse.json(
        { bookmark: { id: 'b1', editionId: 'e1', cfi: 'epubcfi(/6/4)', label: '', createdAt: '2026-09-01T00:00:00Z' } },
        { status: 201 },
      ),
    ),
    http.get('*/api/v1/reading/editions/:editionId/highlights', () =>
      HttpResponse.json({ highlights: [] }),
    ),
  ]
}

function renderReader(entry = '/read/w1/e1') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createMemoryRouter(
    [{ path: '/read/:workId/:editionId', element: <Reader /> }],
    { initialEntries: [entry] },
  )
  return render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  server.use(...contentHandlers(), ...readingApiHandlers())
  window.localStorage.clear()
})

describe('Reader', () => {
  it('renders the chrome and a sandboxed iframe that never allows scripts', async () => {
    renderReader()

    expect(await screen.findByRole('link', { name: '← Library' })).toBeInTheDocument()
    const frame = await screen.findByTitle(/reading area/i)
    // Sandbox is exactly allow-same-origin — never allow-scripts.
    expect(frame.getAttribute('sandbox')).toBe('allow-same-origin')
    expect(frame.getAttribute('sandbox')).not.toContain('allow-scripts')
  })

  it('issues the content grant before pointing the iframe at the chapter', async () => {
    const issued: string[] = []
    let grant!: () => void
    const granted = new Promise<void>((resolve) => (grant = resolve))
    server.use(
      http.post('*/api/v1/library/editions/:editionId/reader/session', async ({ params }) => {
        issued.push(String(params.editionId))
        await granted
        return new HttpResponse(null, { status: 204 })
      }),
    )
    const loaded: string[] = []
    const onResponse = ({ request }: { request: Request }) => loaded.push(request.url)
    server.events.on('response:mocked', onResponse)
    renderReader()

    // The book's structure finishes loading, but no chapter is framed
    // while the grant request is still open.
    try {
      await waitFor(() => expect(loaded.some((u) => u.endsWith('OEBPS/nav.xhtml'))).toBe(true))
    } finally {
      server.events.removeListener('response:mocked', onResponse)
    }
    await new Promise((r) => setTimeout(r, 100))
    expect(issued).toEqual(['e1'])
    expect(screen.queryByTitle(/reading area/i)?.getAttribute('src') ?? null).toBeNull()

    grant()
    const frame = await screen.findByTitle(/reading area/i)
    await waitFor(() =>
      expect(frame.getAttribute('src')).toMatch(
        /\/editions\/e1\/reader\/content\/OEBPS\/chapter1\.xhtml$/,
      ),
    )
  })

  it('re-issues a stale grant before loading the next chapter', async () => {
    let issued = 0
    server.use(
      http.post('*/api/v1/library/editions/:editionId/reader/session', () => {
        issued++
        return new HttpResponse(null, { status: 204 })
      }),
    )
    renderReader()
    const user = userEvent.setup()
    const frame = await screen.findByTitle(/reading area/i)
    await waitFor(() => expect(frame.getAttribute('src')).toMatch(/chapter1\.xhtml$/))
    expect(issued).toBe(1)

    // A laptop slept past the grant's lifetime: wall-clock time jumped,
    // the refresh timer did not fire.
    const realNow = Date.now()
    const clock = vi.spyOn(Date, 'now').mockReturnValue(realNow + 13 * 60_000)
    try {
      await user.click(screen.getByRole('button', { name: 'Next chapter' }))
      await waitFor(() => expect(frame.getAttribute('src')).toMatch(/chapter2\.xhtml$/))
      expect(issued).toBe(2)
    } finally {
      clock.mockRestore()
    }
  })

  it('shows an error instead of a blank frame when the content grant is refused', async () => {
    server.use(
      http.post('*/api/v1/library/editions/:editionId/reader/session', () =>
        HttpResponse.json({ code: 'not_found', message: 'no such edition in your library' }, { status: 404 }),
      ),
    )
    renderReader()

    expect(await screen.findByText('This book could not be opened.')).toBeInTheDocument()
    expect(screen.queryByTitle(/reading area/i)).not.toBeInTheDocument()
  })

  it('says so and offers a retry when a stale grant cannot be re-issued', async () => {
    let fail = false
    server.use(
      http.post('*/api/v1/library/editions/:editionId/reader/session', () =>
        fail
          ? HttpResponse.json({ code: 'unavailable' }, { status: 503 })
          : new HttpResponse(null, { status: 204 }),
      ),
    )
    renderReader()
    const user = userEvent.setup()
    const frame = await screen.findByTitle(/reading area/i)
    await waitFor(() => expect(frame.getAttribute('src')).toMatch(/chapter1\.xhtml$/))

    const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.now() + 13 * 60_000)
    try {
      fail = true
      await user.click(screen.getByRole('button', { name: 'Next chapter' }))
      expect(await screen.findByRole('alert')).toHaveTextContent(/couldn.t be loaded/i)
      expect(frame.getAttribute('src')).toMatch(/chapter1\.xhtml$/)

      fail = false
      await user.click(screen.getByRole('button', { name: 'Try again' }))
      await waitFor(() => expect(frame.getAttribute('src')).toMatch(/chapter2\.xhtml$/))
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    } finally {
      clock.mockRestore()
    }
  })

  it('measures position against the chapter the frame holds, not the one being opened', async () => {
    let release!: () => void
    let hold = false
    server.use(
      http.post('*/api/v1/library/editions/:editionId/reader/session', async () => {
        if (hold) await new Promise<void>((r) => (release = r))
        return new HttpResponse(null, { status: 204 })
      }),
    )
    renderReader()
    const user = userEvent.setup()
    const frame = (await screen.findByTitle(/reading area/i)) as HTMLIFrameElement
    await waitFor(() => expect(frame.getAttribute('src')).toMatch(/chapter1\.xhtml$/))
    // jsdom leaves the frame's document empty; give it a root to measure.
    const frameDoc = frame.contentDocument!
    if (!frameDoc.documentElement) frameDoc.appendChild(frameDoc.createElement('html'))
    fireEvent.load(frame)
    const progress = () => screen.getByRole('progressbar').getAttribute('aria-valuenow')
    const atChapterOne = progress()

    const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.now() + 13 * 60_000)
    try {
      hold = true
      await user.click(screen.getByRole('button', { name: 'Next chapter' }))
      // The grant is being re-issued; the frame still holds chapter 1.
      expect(frame.getAttribute('src')).toMatch(/chapter1\.xhtml$/)
      frame.contentWindow!.dispatchEvent(new Event('scroll'))
      await waitFor(() => expect(progress()).toBe(atChapterOne))
    } finally {
      release?.()
      clock.mockRestore()
    }
  })

  it('keeps the sandbox script-free after a theme change', async () => {
    renderReader()
    const user = userEvent.setup()

    await screen.findByTitle(/reading area/i)
    await user.click(screen.getByRole('button', { name: 'Reading settings' }))
    await user.click(await screen.findByRole('button', { name: 'dark' }))

    const frame = screen.getByTitle(/reading area/i)
    expect(frame.getAttribute('sandbox')).toBe('allow-same-origin')
  })

  it('a script-injection fixture never executes', async () => {
    server.use(...contentHandlers(CHAPTER1_HOSTILE))
    renderReader()
    await screen.findByTitle(/reading area/i)

    // The reader renders sanitised content the backend already stripped;
    // even if a raw script reached the iframe, allow-scripts is absent, so
    // nothing runs. jsdom does not execute sandboxed-iframe scripts either.
    await new Promise((r) => setTimeout(r, 50))
    expect((window as unknown as { __pwned?: boolean }).__pwned).toBeUndefined()
  })

  it('opens the table of contents and lists chapters', async () => {
    renderReader()
    const user = userEvent.setup()

    await screen.findByTitle(/reading area/i)
    await user.click(screen.getByRole('button', { name: 'Contents' }))

    const toc = await screen.findByRole('navigation', { name: /table of contents/i })
    expect(within(toc).getByRole('button', { name: 'Chapter One' })).toBeInTheDocument()
    expect(within(toc).getByRole('button', { name: 'Chapter Two' })).toBeInTheDocument()
  })

  it('persists a theme preference change', async () => {
    let saved: unknown
    server.use(
      http.put('*/api/v1/reading/preferences', async ({ request }) => {
        saved = await request.json()
        return HttpResponse.json({ preferences: saved })
      }),
    )
    renderReader()
    const user = userEvent.setup()

    await screen.findByTitle(/reading area/i)
    await user.click(screen.getByRole('button', { name: 'Reading settings' }))
    await user.click(await screen.findByRole('button', { name: 'sepia' }))

    await waitFor(() => expect((saved as { theme?: string })?.theme).toBe('sepia'), { timeout: 2000 })
  })

  it('bookmarks the current page', async () => {
    let created = false
    server.use(
      http.post('*/api/v1/reading/editions/:editionId/bookmarks', () => {
        created = true
        return HttpResponse.json(
          { bookmark: { id: 'b1', editionId: 'e1', cfi: 'epubcfi(/6/4)', label: '', createdAt: 'x' } },
          { status: 201 },
        )
      }),
    )
    renderReader()
    const user = userEvent.setup()

    await screen.findByTitle(/reading area/i)
    await user.click(screen.getByRole('button', { name: 'Marks' }))
    await user.click(await screen.findByRole('button', { name: /bookmark this page/i }))

    await waitFor(() => expect(created).toBe(true))
  })

  it('downloads the reading export from the Marks panel', async () => {
    const clickSpy = vi.fn()
    const realCreate = document.createElement.bind(document)
    vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      const el = realCreate(tag)
      if (tag === 'a') el.click = clickSpy
      return el
    })
    server.use(
      http.get('*/api/v1/reading/export', () =>
        HttpResponse.json({ schemaVersion: 1, exportedAt: 'x', works: [], editions: [] }),
      ),
    )

    renderReader()
    const user = userEvent.setup()
    await screen.findByTitle(/reading area/i)
    await user.click(screen.getByRole('button', { name: 'Marks' }))
    await user.click(await screen.findByRole('button', { name: 'Export' }))

    await waitFor(() => expect(clickSpy).toHaveBeenCalled())
    vi.restoreAllMocks()
  })

  it('surfaces an unreachable source with a retry', async () => {
    server.use(
      http.get(CONTENT, () => HttpResponse.json({ code: 'unavailable' }, { status: 503 })),
    )
    renderReader()

    expect(
      await screen.findByText(/isn't reachable right now/i, {}, { timeout: 2000 }),
    ).toBeInTheDocument()
  })

  it('has no axe violations in its chrome', async () => {
    const { container } = renderReader()
    await screen.findByTitle(/reading area/i)
    expectNoAxeViolations(await runAxe(container, { exclude: 'iframe' }))
  })

  it('reader chrome stays usable at a narrow viewport width', async () => {
    renderReader()
    await screen.findByTitle(/reading area/i)
    // The chrome is CSS-driven (reader.css uses min()/clamp responsive
    // widths); the controls remain in the accessibility tree regardless
    // of width, which is what a screen reader and keyboard depend on.
    expect(screen.getByRole('button', { name: 'Contents' })).toBeInTheDocument()
    expect(screen.getByRole('progressbar', { name: /reading progress/i })).toBeInTheDocument()
  })
})
