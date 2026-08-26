import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { Input } from './Input'

describe('Input', () => {
  it('associates the label with the input', () => {
    render(<Input label="Title" />)
    expect(screen.getByLabelText('Title')).toBeInTheDocument()
  })

  it('is focusable and operable by keyboard alone', async () => {
    const user = userEvent.setup()
    render(<Input label="Title" />)
    await user.tab()
    const input = screen.getByLabelText('Title')
    expect(input).toHaveFocus()
    await user.keyboard('hello')
    expect(input).toHaveValue('hello')
  })

  it('renders disabled', () => {
    render(<Input label="Title" disabled />)
    expect(screen.getByLabelText('Title')).toBeDisabled()
  })

  it('renders an error state with aria-invalid and an announced message', () => {
    render(<Input label="Title" error="Title is required" />)
    const input = screen.getByLabelText('Title')
    expect(input).toHaveAttribute('aria-invalid', 'true')
    const message = screen.getByRole('alert')
    expect(message).toHaveTextContent('Title is required')
    expect(input).toHaveAttribute('aria-describedby', expect.stringContaining(message.id))
  })

  it('renders a hint when there is no error', () => {
    render(<Input label="Title" hint="As it appears on the cover" />)
    expect(screen.getByText('As it appears on the cover')).toBeInTheDocument()
  })

  it('has zero axe violations, including in the error state', async () => {
    const { container } = render(<Input label="Title" error="Title is required" />)
    expectNoAxeViolations(await runAxe(container))
  })
})
