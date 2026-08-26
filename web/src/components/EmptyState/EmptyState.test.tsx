import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { EmptyState } from './EmptyState'

describe('EmptyState', () => {
  it('renders the title and optional description', () => {
    render(<EmptyState title="No books yet" description="Add a source to get started" />)
    expect(screen.getByText('No books yet')).toBeInTheDocument()
    expect(screen.getByText('Add a source to get started')).toBeInTheDocument()
  })

  it('hides a decorative icon from the accessibility tree', () => {
    render(<EmptyState title="No books yet" icon={<svg data-testid="icon" />} />)
    expect(screen.getByTestId('icon').closest('[aria-hidden="true"]')).not.toBeNull()
  })

  it('exposes a keyboard-operable action when provided', async () => {
    const onClick = vi.fn()
    const user = userEvent.setup()
    render(<EmptyState title="No books yet" action={{ label: 'Add source', onClick }} />)
    await user.tab()
    expect(screen.getByRole('button', { name: 'Add source' })).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(onClick).toHaveBeenCalledTimes(1)
  })

  it('has zero axe violations', async () => {
    const { container } = render(
      <EmptyState
        title="No books yet"
        description="Add a source to get started"
        action={{ label: 'Add source', onClick: vi.fn() }}
      />,
    )
    expectNoAxeViolations(await runAxe(container))
  })
})
