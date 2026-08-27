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
    queryFn: () => getJson<{ ok: true }>('/api/v1/probe'),
  })
  return <QueryResult query={query}>{() => <p>loaded</p>}</QueryResult>
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
        return HttpResponse.json({ ok: true })
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
})
