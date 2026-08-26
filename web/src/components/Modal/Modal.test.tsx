import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { Modal } from './Modal'

describe('Modal', () => {
  it('renders its title and content when open', () => {
    render(
      <Modal open title="Edit collection" description="Rename or delete this collection">
        <button>Save</button>
      </Modal>,
    )
    expect(screen.getByRole('dialog', { name: 'Edit collection' })).toBeInTheDocument()
    expect(screen.getByText('Rename or delete this collection')).toBeInTheDocument()
  })

  it('marks the rest of the page inert while open', () => {
    render(
      <>
        <button>Outside</button>
        <Modal open title="Edit collection" />
      </>,
    )
    // Radix marks the ancestor wrapper aria-hidden, not the button itself,
    // which is also why an accessibility-tree query (getByRole) can no
    // longer find it — query past that filtering to check the DOM directly.
    const outside = screen.getByText('Outside', { selector: 'button' })
    expect(outside.closest('[aria-hidden="true"]')).not.toBeNull()
  })

  it('traps focus inside the dialog', async () => {
    const user = userEvent.setup()
    render(
      <Modal open title="Edit collection">
        <button>Save</button>
      </Modal>,
    )
    const close = screen.getByRole('button', { name: 'Close' })
    const save = screen.getByRole('button', { name: 'Save' })
    // Radix auto-focuses the first focusable element in the content on open.
    expect(save).toHaveFocus()
    await user.tab()
    expect(close).toHaveFocus()
    await user.tab()
    expect(save).toHaveFocus()
  })

  it('closes on Escape', async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(<Modal open onOpenChange={onOpenChange} title="Edit collection" />)
    await user.keyboard('{Escape}')
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('returns focus to the trigger on close', async () => {
    const user = userEvent.setup()
    function Harness() {
      const [open, setOpen] = useState(false)
      return (
        <Modal
          open={open}
          onOpenChange={setOpen}
          trigger={<button>Open</button>}
          title="Edit collection"
        />
      )
    }
    render(<Harness />)
    const trigger = screen.getByRole('button', { name: 'Open' })
    await user.click(trigger)
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(trigger).toHaveFocus()
  })

  it('has zero axe violations', async () => {
    const { container } = render(
      <Modal open title="Edit collection" description="Rename or delete this collection">
        <button>Save</button>
      </Modal>,
    )
    expectNoAxeViolations(await runAxe(container))
  })
})
