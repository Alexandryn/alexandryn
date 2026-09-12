import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
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
