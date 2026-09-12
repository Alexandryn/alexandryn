import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { server } from '../../mocks/node'
import { ResetPasswordScreen } from './ResetPasswordScreen'

function LoginProbe() {
  const loc = useLocation()
  return <div data-testid="login-page">{JSON.stringify(loc.state)}</div>
}

function renderScreen(entry: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[entry]}>
        <Routes>
          <Route path="/reset-password" element={<ResetPasswordScreen />} />
          <Route path="/login" element={<LoginProbe />} />
          <Route path="/forgot-password" element={<div data-testid="forgot-page">forgot</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('ResetPasswordScreen', () => {
  it('refuses to show the form without a token', () => {
    renderScreen('/reset-password')
    expect(screen.getByRole('alert')).toHaveTextContent('This page needs a reset link')
    expect(screen.queryByLabelText('New password')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Request a reset link' })).toHaveAttribute(
      'href',
      '/forgot-password',
    )
  })

  it('rejects mismatched passwords before calling the API', async () => {
    let called = false
    server.use(
      http.post('*/api/v1/auth/password-reset/confirm', () => {
        called = true
        return HttpResponse.json({ success: true })
      }),
    )
    const user = userEvent.setup()
    renderScreen('/reset-password?token=abc')

    await user.type(screen.getByLabelText('New password'), 'longenough1')
    await user.type(screen.getByLabelText('Confirm new password'), 'different99')
    await user.click(screen.getByRole('button', { name: 'Save new password' }))

    expect(screen.getByRole('alert')).toHaveTextContent('do not match')
    expect(called).toBe(false)
  })

  it('confirms the reset and returns to login with a flag', async () => {
    server.use(
      http.post('*/api/v1/auth/password-reset/confirm', async ({ request }) => {
        const body = (await request.json()) as { token: string; newPassword: string }
        expect(body.token).toBe('tok-123')
        return HttpResponse.json({ success: true })
      }),
    )
    const user = userEvent.setup()
    renderScreen('/reset-password?token=tok-123')

    await user.type(screen.getByLabelText('New password'), 'brand-new-pass')
    await user.type(screen.getByLabelText('Confirm new password'), 'brand-new-pass')
    await user.click(screen.getByRole('button', { name: 'Save new password' }))

    await waitFor(() => expect(screen.getByTestId('login-page')).toBeInTheDocument())
    expect(screen.getByTestId('login-page')).toHaveTextContent('"passwordReset":true')
  })

  it('surfaces an expired-link error', async () => {
    server.use(
      http.post('*/api/v1/auth/password-reset/confirm', () =>
        HttpResponse.json({ code: 'invalid', message: 'token expired' }, { status: 400 }),
      ),
    )
    const user = userEvent.setup()
    renderScreen('/reset-password?token=old')

    await user.type(screen.getByLabelText('New password'), 'brand-new-pass')
    await user.type(screen.getByLabelText('Confirm new password'), 'brand-new-pass')
    await user.click(screen.getByRole('button', { name: 'Save new password' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('invalid or has expired')
  })
})
