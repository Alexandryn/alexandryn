import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { renderWithProviders } from '../../test/renderWithProviders'
import { DiagnosticsScreen } from './DiagnosticsScreen'

describe('DiagnosticsScreen', () => {
  it('renders system uptime, runtime memory statistics, and version details', async () => {
    server.use(
      http.get('*/api/v1/diagnostics', () =>
        HttpResponse.json({
          uptime_seconds: 3665,
          version: {
            commit: 'a1b2c3d4',
            build_time: '2026-09-28T12:00:00Z',
          },
          runtime: {
            goroutines: 18,
            heap_alloc_bytes: 8 * 1024 * 1024,
            total_alloc_bytes: 32 * 1024 * 1024,
            gc_cycles: 4,
          },
        }),
      ),
    )

    renderWithProviders(<DiagnosticsScreen />)

    // Heading
    expect(screen.getByRole('heading', { name: 'System Diagnostics' })).toBeInTheDocument()

    // Uptime formatted: 3665s = 1h 1m
    expect(await screen.findByText('1h 1m')).toBeInTheDocument()

    // Active Goroutines
    expect(screen.getByText('18')).toBeInTheDocument()

    // Heap allocation: 8 MiB
    expect(screen.getByText('8.0 MiB')).toBeInTheDocument()

    // Total allocation: 32 MiB
    expect(screen.getByText('32.0 MiB')).toBeInTheDocument()

    // GC Cycles
    expect(screen.getByText('4')).toBeInTheDocument()

    // Version details
    expect(screen.getByText('a1b2c3d4')).toBeInTheDocument()
    expect(screen.getByText('2026-09-28T12:00:00Z')).toBeInTheDocument()

    // Refresh button
    expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument()
  })

  it('renders error notice with retry button when diagnostics query fails', async () => {
    server.use(
      http.get('*/api/v1/diagnostics', () =>
        HttpResponse.json({ message: 'Forbidden' }, { status: 403 }),
      ),
    )

    renderWithProviders(<DiagnosticsScreen />)

    expect(await screen.findByText('Forbidden')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument()
  })
})
