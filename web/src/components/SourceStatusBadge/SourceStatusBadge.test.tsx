import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { SourceStatusBadge } from './SourceStatusBadge'

describe('SourceStatusBadge (FR-3)', () => {
  it('renders Reachable state with success tone', () => {
    render(
      <SourceStatusBadge
        health={{ status: 'reachable', checkedAt: '2026-08-31T12:00:00Z', detail: null }}
      />,
    )
    expect(screen.getByText('Reachable')).toBeInTheDocument()
  })

  it('renders in-flight / checking state', () => {
    render(
      <SourceStatusBadge
        health={{ status: 'unknown', checkedAt: null, detail: null }}
        isChecking={true}
      />,
    )
    expect(screen.getByText('Checking...')).toBeInTheDocument()
  })

  const detailCases: [string, string][] = [
    ['timeout', "Can't connect — timed out"],
    ['connection-refused', "Can't connect — connection refused"],
    ['auth-rejected', 'Wrong username or password'],
    ['http-4xx', 'This source returned an error'],
    ['http-5xx', 'This source is having a problem right now'],
    ['http-3xx-unsupported', "This source tried to redirect, which isn't supported"],
    ['unparseable-response', 'Received an unexpected response'],
    ['path-not-found', 'Folder not found'],
    ['path-not-readable', "Folder isn't readable"],
  ]

  it.each(detailCases)('renders unreachable detail %s as %s', (detail, expectedText) => {
    render(
      <SourceStatusBadge
        health={{
          status: 'unreachable',
          checkedAt: '2026-08-31T12:00:00Z',
          detail: detail as any,
        }}
      />,
    )
    expect(screen.getByText(expectedText)).toBeInTheDocument()
  })

  it('offers "Check again" button when unreachable and onCheckAgain provided', async () => {
    const handleCheckAgain = vi.fn()
    const user = userEvent.setup()

    render(
      <SourceStatusBadge
        health={{ status: 'unreachable', checkedAt: '2026-08-31T12:00:00Z', detail: 'timeout' }}
        onCheckAgain={handleCheckAgain}
      />,
    )

    const button = screen.getByRole('button', { name: 'Check again' })
    expect(button).toBeInTheDocument()

    await user.click(button)
    expect(handleCheckAgain).toHaveBeenCalledTimes(1)
  })
})
