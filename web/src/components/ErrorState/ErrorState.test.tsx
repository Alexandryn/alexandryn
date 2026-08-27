import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { ErrorState } from './ErrorState'

describe('ErrorState', () => {
  it('leads with the message and shows the correlation ID visibly', () => {
    render(
      <ErrorState
        title="Couldn't load your library"
        code="unavailable"
        correlationId="corr-12345"
      />,
    )
    expect(screen.getByText("Couldn't load your library")).toBeInTheDocument()
    const ref = screen.getByTestId('correlation-id')
    expect(ref).toBeVisible()
    expect(ref).toHaveTextContent('corr-12345')
    expect(screen.getByText('unavailable')).toBeInTheDocument()
  })

  it('omits the technical line entirely when there is no code or id', () => {
    render(<ErrorState title="That collection no longer exists" />)
    expect(screen.queryByTestId('correlation-id')).not.toBeInTheDocument()
  })

  it('calls onRetry from the retry button', async () => {
    const onRetry = vi.fn()
    render(<ErrorState title="Failed" onRetry={onRetry} />)
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('is an alert and has no axe violations', async () => {
    const { container } = render(
      <ErrorState title="Failed" code="internal" correlationId="corr-1" onRetry={() => {}} />,
    )
    expect(screen.getByRole('alert')).toBeInTheDocument()
    expectNoAxeViolations(await runAxe(container))
  })
})
