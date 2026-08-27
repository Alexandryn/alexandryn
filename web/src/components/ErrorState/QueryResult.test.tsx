import { useQuery } from '@tanstack/react-query'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { getJson } from '../../data/http'
import { server } from '../../mocks/node'
import { renderWithProviders } from '../../test/renderWithProviders'
import { QueryResult } from './QueryResult'

function Probe() {
  const query = useQuery({
    queryKey: ['probe'],
    queryFn: () => getJson<{ label: string }>('/api/v1/probe'),
  })
  return <QueryResult query={query}>{(data) => <p>{data.label}</p>}</QueryResult>
}

describe('QueryResult', () => {
  it('renders the API error with its correlation ID and a working retry', async () => {
    let attempt = 0
    server.use(
      http.get('*/api/v1/probe', () => {
        attempt += 1
        if (attempt === 1) {
          return HttpResponse.json(
            {
              code: 'unavailable',
              message: 'The server is restarting.',
              correlationId: 'corr-probe-777',
            },
            { status: 503 },
          )
        }
        return HttpResponse.json({ label: 'loaded' })
      }),
    )

    renderWithProviders(<Probe />)

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByTestId('correlation-id')).toHaveTextContent('corr-probe-777')
    expect(screen.getByText('The server is restarting.')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))

    await waitFor(() => expect(screen.getByText('loaded')).toBeInTheDocument())
    expect(attempt).toBe(2)
  })

  it('keeps already-loaded data visible when a later refetch fails (stale-while-revalidate)', async () => {
    let attempt = 0
    server.use(
      http.get('*/api/v1/probe', () => {
        attempt += 1
        if (attempt === 1) return HttpResponse.json({ label: 'first load' })
        return HttpResponse.json(
          { code: 'unavailable', message: 'gone', correlationId: 'corr-stale-1' },
          { status: 503 },
        )
      }),
    )

    const { queryClient } = renderWithProviders(<Probe />)
    expect(await screen.findByText('first load')).toBeInTheDocument()

    void queryClient.refetchQueries({ queryKey: ['probe'] })

    // A non-blocking notice appears once the refetch fails...
    const notice = await screen.findByRole('status')
    expect(notice).toHaveTextContent(/couldn't refresh/i)
    expect(screen.getByTestId('correlation-id')).toHaveTextContent('corr-stale-1')
    // ...and the already-loaded data is never blanked.
    expect(screen.getByText('first load')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(attempt).toBe(2)
  })
})
