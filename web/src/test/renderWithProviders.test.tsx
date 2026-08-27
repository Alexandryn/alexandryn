import { useQuery } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import { renderWithProviders } from './renderWithProviders'

function Probe() {
  const { data } = useQuery({
    queryKey: ['probe'],
    queryFn: () => Promise.resolve('ok'),
  })
  return <p>{data ?? 'pending'}</p>
}

it('a useQuery component throws with no QueryClient in context', () => {
  expect(() => render(<Probe />)).toThrow()
})

it('renderWithProviders supplies a QueryClient', async () => {
  renderWithProviders(<Probe />)
  expect(await screen.findByText('ok')).toBeInTheDocument()
})

it('renderWithProviders can also mount a memory router', () => {
  renderWithProviders(<p>routed</p>, { routerEntries: ['/anything'] })
  expect(screen.getByText('routed')).toBeInTheDocument()
})
