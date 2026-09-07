import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { renderWithProviders } from '../../test/renderWithProviders'
import { ActivityScreen } from './Activity'
import { Sidebar } from '../../app/shell/Sidebar'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import type { SystemEvent } from '../../data/activity'

const mockEvents: SystemEvent[] = [
  {
    id: 1,
    event_kind: 'job.running',
    job_id: 'job-active-1',
    created_at: '2026-09-07T03:30:00Z',
    purge_at: '2026-10-07T03:30:00Z',
    payload: {
      title: 'Moby Dick',
      author: 'Herman Melville',
      source_name: 'Gutenberg',
      format: 'EPUB',
      progress_percent: 65,
      progress_text: '65% · 2.1 MB of 3.4 MB',
    },
  },
  {
    id: 2,
    event_kind: 'job.enqueued',
    job_id: 'job-queued-1',
    created_at: '2026-09-07T03:25:00Z',
    purge_at: '2026-10-07T03:25:00Z',
    payload: {
      title: 'War and Peace',
      source_name: 'Standard Ebooks',
      format: 'EPUB',
      progress_text: 'Queued · position 1',
    },
  },
  {
    id: 3,
    event_kind: 'job.failed',
    job_id: 'job-failed-1',
    created_at: '2026-09-07T03:20:00Z',
    purge_at: '2026-10-07T03:20:00Z',
    payload: {
      title: 'Frankenstein',
      source_name: 'Archive.org',
      error: 'Source unavailable · connection timed out',
    },
  },
  {
    id: 4,
    event_kind: 'job.completed',
    job_id: 'job-done-1',
    created_at: '2026-09-07T03:10:00Z',
    purge_at: '2026-10-07T03:10:00Z',
    payload: {
      title: 'The Great Gatsby',
      source_name: 'Gutenberg',
      format: 'EPUB',
      work_id: 'work-123',
    },
  },
]

describe('ActivityScreen (Phase 15, frontend-activity-screen.md)', () => {
  it('renders empty state when there are no events', async () => {
    server.use(http.get('*/api/v1/activity/events', () => HttpResponse.json({ events: [] })))

    renderWithProviders(<ActivityScreen />, { routerEntries: ['/activity'] })

    expect(await screen.findByRole('heading', { name: 'Activity', level: 1 })).toBeInTheDocument()
    expect(
      screen.getByText('What Alexandryn is fetching from your sources, and what you have been reading.'),
    ).toBeInTheDocument()
    expect(
      await screen.findByText('No recent activity. Books you import or acquire from sources will appear here.'),
    ).toBeInTheDocument()

    // Gate 0 G0-1: Reading tab label must NOT be rendered
    expect(screen.queryByText('Reading')).not.toBeInTheDocument()
  })

  it('renders ACTIVE, QUEUED, FAILED, and COMPLETED sections with correct metadata', async () => {
    server.use(http.get('*/api/v1/activity/events', () => HttpResponse.json({ events: mockEvents })))

    renderWithProviders(<ActivityScreen />, { routerEntries: ['/activity'] })

    // Section headings (h2)
    expect(await screen.findByRole('heading', { name: 'ACTIVE', level: 2 })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'QUEUED', level: 2 })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'FAILED', level: 2 })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'COMPLETED', level: 2 })).toBeInTheDocument()

    // Active item
    expect(screen.getByText('Moby Dick')).toBeInTheDocument()
    expect(screen.getByText('Herman Melville')).toBeInTheDocument()
    expect(screen.getByText('65% · 2.1 MB of 3.4 MB')).toBeInTheDocument()

    // Queued item
    expect(screen.getByText('War and Peace')).toBeInTheDocument()
    expect(screen.getByText('Queued · position 1')).toBeInTheDocument()

    // Failed item with error description and Fix source link
    expect(screen.getByText('Frankenstein')).toBeInTheDocument()
    expect(screen.getByText('Source unavailable · connection timed out')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Fix source' })).toHaveAttribute('href', '/sources')

    // Completed item with Read link
    expect(screen.getByText('The Great Gatsby')).toBeInTheDocument()
    expect(screen.getByText('Added to library')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Read' })).toHaveAttribute('href', '/book/work-123')
  })

  it('triggers Pause all, Cancel, Retry, and Clear action requests', async () => {
    let pauseAllCalled = false
    let cancelledId = ''
    let retriedId = ''
    let clearCalled = false

    server.use(
      http.get('*/api/v1/activity/events', () => HttpResponse.json({ events: mockEvents })),
      http.post('*/api/v1/activity/pause-all', () => {
        pauseAllCalled = true
        return HttpResponse.json({ paused: true })
      }),
      http.post('*/api/v1/activity/jobs/:id/cancel', ({ params }) => {
        cancelledId = params.id as string
        return HttpResponse.json({ cancelled: true })
      }),
      http.post('*/api/v1/activity/jobs/:id/retry', ({ params }) => {
        retriedId = params.id as string
        return HttpResponse.json({ new_job_id: 'job-new-1' })
      }),
      http.post('*/api/v1/activity/jobs/clear-completed', () => {
        clearCalled = true
        return HttpResponse.json({ cleared_count: 1 })
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<ActivityScreen />, { routerEntries: ['/activity'] })

    // 1. Pause all
    const pauseAllBtn = await screen.findByRole('button', { name: 'Pause all' })
    await user.click(pauseAllBtn)
    expect(pauseAllCalled).toBe(true)

    // 2. Cancel queued job
    const cancelBtn = screen.getByRole('button', { name: 'Cancel' })
    await user.click(cancelBtn)
    expect(cancelledId).toBe('job-queued-1')

    // 3. Retry failed job
    const retryBtn = screen.getByRole('button', { name: 'Retry' })
    await user.click(retryBtn)
    expect(retriedId).toBe('job-failed-1')

    // 4. Clear completed jobs
    const clearBtn = screen.getByRole('button', { name: 'Clear' })
    await user.click(clearBtn)
    expect(clearCalled).toBe(true)
  })

  it('renders indicator dot badge on Activity navigation rail item when active or failed items exist', async () => {
    server.use(http.get('*/api/v1/activity/events', () => HttpResponse.json({ events: mockEvents })))

    renderWithProviders(<Sidebar />, { routerEntries: ['/library'] })

    // Indicator badge dot exists with accessible name (FR-2, FR-3)
    expect(await screen.findByLabelText('Activity — action required')).toBeInTheDocument()
  })

  it('does not render indicator badge when only completed items or queue is empty', async () => {
    server.use(
      http.get('*/api/v1/activity/events', () =>
        HttpResponse.json({
          events: [
            {
              id: 10,
              event_kind: 'job.completed',
              job_id: 'j-comp',
              created_at: '2026-09-07T03:00:00Z',
              purge_at: '2026-10-07T03:00:00Z',
              payload: { title: 'Done Book' },
            },
          ],
        }),
      ),
    )

    renderWithProviders(<Sidebar />, { routerEntries: ['/library'] })

    await waitFor(() => {
      expect(screen.queryByLabelText('Activity — action required')).not.toBeInTheDocument()
    })
  })

  it('passes axe-core accessibility audit with all sections rendered', async () => {
    server.use(http.get('*/api/v1/activity/events', () => HttpResponse.json({ events: mockEvents })))

    const { container } = renderWithProviders(<ActivityScreen />, { routerEntries: ['/activity'] })
    await screen.findByRole('heading', { name: 'ACTIVE', level: 2 })

    const violations = await runAxe(container)
    expectNoAxeViolations(violations)
  })
})

