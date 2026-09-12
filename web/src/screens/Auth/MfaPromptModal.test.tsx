import { QueryClientProvider, QueryClient } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { MfaPromptModal } from './MfaPromptModal'

function wrap(ui: ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>)
}

// The MFA prompt modal is built on the shared Radix Modal (dialog role,
// focus trap, Escape key handling, focus return, and OTP autofill hints).
describe('MfaPromptModal accessibility', () => {
  it('is an accessible dialog with an accessible name', () => {
    wrap(<MfaPromptModal mfaTicket="t" onSuccess={vi.fn()} onCancel={vi.fn()} />)
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAccessibleName(/two-factor authentication/i)
  })

  it('moves focus into the dialog and marks the code field for OTP autofill', () => {
    wrap(<MfaPromptModal mfaTicket="t" onSuccess={vi.fn()} onCancel={vi.fn()} />)
    const code = screen.getByLabelText(/authenticator code/i)
    expect(code).toHaveFocus()
    expect(code).toHaveAttribute('autocomplete', 'one-time-code')
    expect(code).toHaveAttribute('inputmode', 'numeric')
  })

  it('closes on Escape', async () => {
    const onCancel = vi.fn()
    wrap(<MfaPromptModal mfaTicket="t" onSuccess={vi.fn()} onCancel={onCancel} />)
    await userEvent.keyboard('{Escape}')
    expect(onCancel).toHaveBeenCalled()
  })
})
