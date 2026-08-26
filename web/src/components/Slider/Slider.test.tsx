import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { Slider } from './Slider'

describe('Slider', () => {
  it('renders with the correct ARIA value state', () => {
    render(<Slider label="Font size" defaultValue={[40]} min={0} max={100} />)
    const thumb = screen.getByRole('slider', { name: 'Font size' })
    expect(thumb).toHaveAttribute('aria-valuenow', '40')
    expect(thumb).toHaveAttribute('aria-valuemin', '0')
    expect(thumb).toHaveAttribute('aria-valuemax', '100')
  })

  it('is operable by keyboard alone (Tab + Arrow keys adjust value)', async () => {
    const onValueChange = vi.fn()
    const user = userEvent.setup()
    render(
      <Slider label="Font size" defaultValue={[40]} min={0} max={100} onValueChange={onValueChange} />,
    )
    await user.tab()
    expect(screen.getByRole('slider')).toHaveFocus()
    await user.keyboard('{ArrowRight}')
    expect(onValueChange).toHaveBeenCalledWith([41])
  })

  it('renders disabled and removes it from the tab order', () => {
    render(<Slider label="Font size" defaultValue={[40]} disabled />)
    const thumb = screen.getByRole('slider')
    expect(thumb).toHaveAttribute('data-disabled')
    expect(thumb).not.toHaveAttribute('tabindex')
  })

  it('has zero axe violations', async () => {
    const { container } = render(<Slider label="Font size" defaultValue={[40]} />)
    expectNoAxeViolations(await runAxe(container))
  })
})
