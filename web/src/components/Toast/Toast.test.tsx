import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { Toast, ToastProvider, ToastViewport } from './Toast'

function renderToast(props: Partial<React.ComponentProps<typeof Toast>> = {}) {
  return render(
    <ToastProvider>
      <Toast title="Import complete" open duration={Infinity} {...props} />
      <ToastViewport />
    </ToastProvider>,
  )
}

describe('Toast', () => {
  it('renders as an announced status region', async () => {
    renderToast({ description: '12 books added' })
    // Radix defers the hidden announcer's text to the next animation frame
    // (so it isn't announced twice with the visible content) — wait for it.
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Import complete'))
    const toast = screen.getByRole('status')
    expect(toast).toHaveAttribute('aria-live', 'assertive')
    expect(toast).toHaveTextContent('12 books added')
  })

  it('dismisses via a keyboard-operable close control', async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    renderToast({ onOpenChange })
    const close = screen.getByRole('button', { name: 'Dismiss' })
    // The toast surface itself is the first tab stop (so a keyboard user can
    // pause the auto-dismiss timer by focusing it) — the close button is next.
    await user.tab()
    await user.tab()
    expect(close).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('exposes a keyboard-operable action when provided', async () => {
    const onAction = vi.fn()
    const user = userEvent.setup()
    renderToast({ actionLabel: 'Undo', onAction })
    const action = screen.getByRole('button', { name: 'Undo' })
    await user.tab()
    await user.tab()
    expect(action).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(onAction).toHaveBeenCalledTimes(1)
  })

  it('has zero axe violations', async () => {
    const { container } = renderToast({ description: '12 books added', actionLabel: 'Undo', onAction: vi.fn() })
    expectNoAxeViolations(await runAxe(container))
  })
})
