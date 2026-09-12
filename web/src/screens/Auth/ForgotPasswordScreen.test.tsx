import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { server } from '../../mocks/node'
import { ForgotPasswordScreen } from './ForgotPasswordScreen'

function renderScreen() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/forgot-password']}>
        <Routes>
          <Route path="/forgot-password" element={<ForgotPasswordScreen />} />
          <Route path="/login" element={<div data-testid="login-page">login</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('ForgotPasswordScreen', () => {
  it('shows a neutral confirmation regardless of whether the email is registered', async () => {
    let requestedEmail = ''
    server.use(
      http.post('*/api/v1/auth/password-reset/request', async ({ request }) => {
        requestedEmail = ((await request.json()) as { email: string }).email
        return HttpResponse.json({ message: 'if the email is registered, a link has been sent' })
      }),
    )

    const user = userEvent.setup()
    renderScreen()

    await user.type(screen.getByLabelText('Email'), 'someone@example.com')
    await user.click(screen.getByRole('button', { name: 'Send reset link' }))

    expect(requestedEmail).toBe('someone@example.com')
    expect(await screen.findByRole('status')).toHaveTextContent(
      'If that email is registered, a password reset link is on its way',
    )
    // The form is gone; no way to tell whether the address existed.
    expect(screen.queryByLabelText('Email')).not.toBeInTheDocument()
  })

  it('shows an error when the request itself fails', async () => {
    server.use(
      http.post('*/api/v1/auth/password-reset/request', () => HttpResponse.error()),
    )

    const user = userEvent.setup()
    renderScreen()

    await user.type(screen.getByLabelText('Email'), 'someone@example.com')
    await user.click(screen.getByRole('button', { name: 'Send reset link' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not send the reset email')
  })
})
