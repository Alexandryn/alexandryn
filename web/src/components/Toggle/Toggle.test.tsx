import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { Toggle } from './Toggle'

describe('Toggle', () => {
  it('renders unchecked by default with an associated label', () => {
    render(<Toggle label="Dark mode" />)
    const toggle = screen.getByRole('switch', { name: 'Dark mode' })
    expect(toggle).toHaveAttribute('aria-checked', 'false')
  })

  it('reflects the checked state via aria-checked', () => {
    render(<Toggle label="Dark mode" checked onCheckedChange={vi.fn()} />)
    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true')
  })

  it('is operable by keyboard alone (Tab + Space)', async () => {
    const onCheckedChange = vi.fn()
    const user = userEvent.setup()
    render(<Toggle label="Dark mode" onCheckedChange={onCheckedChange} />)
    await user.tab()
    expect(screen.getByRole('switch')).toHaveFocus()
    await user.keyboard(' ')
    expect(onCheckedChange).toHaveBeenCalledWith(true)
  })

  it('renders disabled and blocks toggling', async () => {
    const onCheckedChange = vi.fn()
    const user = userEvent.setup()
    render(<Toggle label="Dark mode" disabled onCheckedChange={onCheckedChange} />)
    const toggle = screen.getByRole('switch')
    expect(toggle).toBeDisabled()
    await user.click(toggle)
    expect(onCheckedChange).not.toHaveBeenCalled()
  })

  it('has zero axe violations', async () => {
    const { container } = render(<Toggle label="Dark mode" />)
    expectNoAxeViolations(await runAxe(container))
  })
})
