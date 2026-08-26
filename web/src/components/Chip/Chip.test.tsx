import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { Chip } from './Chip'

describe('Chip', () => {
  it('renders its text content', () => {
    render(<Chip>Fiction</Chip>)
    expect(screen.getByText('Fiction')).toBeInTheDocument()
  })

  it('renders no remove control when onRemove is absent', () => {
    render(<Chip>Fiction</Chip>)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('exposes a keyboard-operable remove control with an accessible name', async () => {
    const onRemove = vi.fn()
    const user = userEvent.setup()
    render(<Chip onRemove={onRemove}>Fiction</Chip>)
    const remove = screen.getByRole('button', { name: 'Remove Fiction' })
    await user.tab()
    expect(remove).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(onRemove).toHaveBeenCalledTimes(1)
  })

  it('disables the remove control when disabled', () => {
    render(
      <Chip onRemove={vi.fn()} disabled>
        Fiction
      </Chip>,
    )
    expect(screen.getByRole('button', { name: 'Remove Fiction' })).toBeDisabled()
  })

  it('has zero axe violations, with and without a remove control', async () => {
    const { container, rerender } = render(<Chip>Fiction</Chip>)
    expectNoAxeViolations(await runAxe(container))
    rerender(<Chip onRemove={vi.fn()}>Fiction</Chip>)
    expectNoAxeViolations(await runAxe(container))
  })
})
