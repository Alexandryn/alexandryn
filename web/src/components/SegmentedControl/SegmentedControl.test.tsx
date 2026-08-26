import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { SegmentedControl } from './SegmentedControl'

const options = [
  { value: 'grid', label: 'Grid' },
  { value: 'list', label: 'List' },
  { value: 'table', label: 'Table' },
]

describe('SegmentedControl', () => {
  it('renders one radio per option under a named group', () => {
    render(<SegmentedControl aria-label="View" options={options} defaultValue="grid" />)
    const group = screen.getByRole('radiogroup', { name: 'View' })
    expect(group).toBeInTheDocument()
    expect(screen.getAllByRole('radio')).toHaveLength(3)
    expect(screen.getByRole('radio', { name: 'Grid' })).toHaveAttribute('aria-checked', 'true')
  })

  it('is operable by keyboard alone (Tab, then Arrow keys move and select)', async () => {
    const onValueChange = vi.fn()
    const user = userEvent.setup()
    render(
      <SegmentedControl
        aria-label="View"
        options={options}
        defaultValue="grid"
        onValueChange={onValueChange}
      />,
    )
    await user.tab()
    expect(screen.getByRole('radio', { name: 'Grid' })).toHaveFocus()
    // Radix defers the roving-focus move to a macrotask and only commits
    // the selection if the key is still held when that macrotask runs —
    // hold the key (userEvent's `{key>}` syntax) instead of a tap-and-
    // release, the same separation real physical key timing gives it.
    await user.keyboard('{ArrowRight>}')
    await waitFor(() => expect(screen.getByRole('radio', { name: 'List' })).toHaveFocus())
    expect(onValueChange).toHaveBeenCalledWith('list')
    await user.keyboard('{/ArrowRight}')
  })

  it('renders a disabled segment that cannot be selected', async () => {
    const onValueChange = vi.fn()
    const user = userEvent.setup()
    render(
      <SegmentedControl
        aria-label="View"
        options={options}
        defaultValue="grid"
        onValueChange={onValueChange}
        disabled
      />,
    )
    const listItem = screen.getByRole('radio', { name: 'List' })
    await user.click(listItem)
    expect(onValueChange).not.toHaveBeenCalled()
  })

  it('has zero axe violations', async () => {
    const { container } = render(
      <SegmentedControl aria-label="View" options={options} defaultValue="grid" />,
    )
    expectNoAxeViolations(await runAxe(container))
  })
})
