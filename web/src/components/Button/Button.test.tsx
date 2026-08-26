import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { VisuallyHidden } from '../VisuallyHidden/VisuallyHidden'
import { Button } from './Button'

describe('Button', () => {
  it('renders the default (primary) state', () => {
    render(<Button>Save</Button>)
    const button = screen.getByRole('button', { name: 'Save' })
    expect(button).toBeEnabled()
    expect(button).toHaveClass('bg-accent')
  })

  it('renders secondary and ghost variants from the token set', () => {
    const { rerender } = render(<Button variant="secondary">Save</Button>)
    expect(screen.getByRole('button')).toHaveClass('bg-surface-2')
    rerender(<Button variant="ghost">Save</Button>)
    expect(screen.getByRole('button')).toHaveClass('text-accent')
  })

  it('carries a visible focus-ring class using the accent token', () => {
    render(<Button>Save</Button>)
    expect(screen.getByRole('button')).toHaveClass('focus-visible:outline-accent')
  })

  it('renders disabled and blocks activation', async () => {
    const onClick = vi.fn()
    const user = userEvent.setup()
    render(
      <Button disabled onClick={onClick}>
        Save
      </Button>,
    )
    const button = screen.getByRole('button', { name: 'Save' })
    expect(button).toBeDisabled()
    await user.click(button)
    expect(onClick).not.toHaveBeenCalled()
  })

  it('is operable by keyboard alone (Tab + Enter, Tab + Space)', async () => {
    const onClick = vi.fn()
    const user = userEvent.setup()
    render(<Button onClick={onClick}>Save</Button>)
    await user.tab()
    expect(screen.getByRole('button')).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(onClick).toHaveBeenCalledTimes(1)
    await user.keyboard(' ')
    expect(onClick).toHaveBeenCalledTimes(2)
  })

  it('supports an icon-only accessible name via VisuallyHidden', () => {
    render(
      <Button>
        <span aria-hidden="true">+</span>
        <VisuallyHidden>Add book</VisuallyHidden>
      </Button>,
    )
    expect(screen.getByRole('button', { name: 'Add book' })).toBeInTheDocument()
  })

  it('has zero axe violations', async () => {
    const { container } = render(<Button>Save</Button>)
    expectNoAxeViolations(await runAxe(container))
  })
})
