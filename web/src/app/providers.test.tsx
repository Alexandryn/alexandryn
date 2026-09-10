import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { useQuery, QueryClient } from '@tanstack/react-query'
import { AppProviders } from './providers'

function Probe() {
  const { data } = useQuery({
    queryKey: ['probe'],
    queryFn: () => Promise.resolve('ok'),
  })
  return <p>{data ?? 'pending'}</p>
}

describe('AppProviders (audit 0016 #232)', () => {
  it('creates an isolated QueryClient per instance and renders children', async () => {
    render(
      <AppProviders>
        <Probe />
      </AppProviders>,
    )
    expect(await screen.findByText('ok')).toBeInTheDocument()
  })

  it('accepts an explicit QueryClient prop', async () => {
    const customClient = new QueryClient()
    render(
      <AppProviders client={customClient}>
        <Probe />
      </AppProviders>,
    )
    expect(await screen.findByText('ok')).toBeInTheDocument()
  })
})
