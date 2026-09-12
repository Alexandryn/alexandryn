import { screen } from '@testing-library/react'
import { useContext } from 'react'
import { describe, expect, it } from 'vitest'
import { CapabilityContext } from './CapabilityContext'
import { CapabilityProvider } from './CapabilityProvider'
import { renderWithProviders } from '../../test/renderWithProviders'
import { server } from '../../mocks/node'
import { http, HttpResponse } from 'msw'

function Probe() {
  const ctx = useContext(CapabilityContext)
  return <div>status:{ctx?.status}</div>
}

describe('CapabilityProvider', () => {
  it('provides granted state when bootstrap succeeds', async () => {
    server.use(
      http.get('*/api/bootstrap', () =>
        HttpResponse.json({
          capabilities: {
            'library:read': true,
          },
        }),
      ),
    )

    renderWithProviders(
      <CapabilityProvider>
        <Probe />
      </CapabilityProvider>,
    )

    expect(await screen.findByText('status:granted')).toBeInTheDocument()
  })

  it('provides error state when bootstrap fails', async () => {
    server.use(
      http.get('*/api/bootstrap', () =>
        HttpResponse.json({ code: 'internal', message: 'error' }, { status: 500 }),
      ),
    )

    renderWithProviders(
      <CapabilityProvider>
        <Probe />
      </CapabilityProvider>,
    )

    expect(await screen.findByText('status:error')).toBeInTheDocument()
  })
})
