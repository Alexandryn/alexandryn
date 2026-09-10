import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import * as auth from '../../data/auth'
import { SetupScreen } from './SetupScreen'

describe('SetupScreen (audit 0016 #245)', () => {
  it('focuses and marks confirmPassword on password mismatch', async () => {
    const user = userEvent.setup()
    render(
      <MemoryRouter>
        <SetupScreen />
      </MemoryRouter>,
    )

    await user.type(screen.getByLabelText('Admin Username'), 'admin')
    await user.type(screen.getByLabelText('Email Address'), 'admin@example.com')
    await user.type(screen.getByLabelText(/^Password/), 'password123')
    await user.type(screen.getByLabelText('Confirm Password'), 'password456')

    await user.click(screen.getByRole('button', { name: 'Initialize Alexandryn' }))

    const confirmInput = screen.getByLabelText('Confirm Password')
    expect(confirmInput).toHaveFocus()
    expect(confirmInput).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('alert')).toHaveTextContent('Passwords do not match')
  })

  it('focuses and marks password on short password', async () => {
    const user = userEvent.setup()
    render(
      <MemoryRouter>
        <SetupScreen />
      </MemoryRouter>,
    )

    await user.type(screen.getByLabelText('Admin Username'), 'admin')
    await user.type(screen.getByLabelText('Email Address'), 'admin@example.com')
    await user.type(screen.getByLabelText(/^Password/), 'short')
    await user.type(screen.getByLabelText('Confirm Password'), 'short')

    await user.click(screen.getByRole('button', { name: 'Initialize Alexandryn' }))

    const passwordInput = screen.getByLabelText(/^Password/)
    expect(passwordInput).toHaveFocus()
    expect(passwordInput).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Password must be at least 8 characters long',
    )
  })

  it('submits setup admin successfully on valid form', async () => {
    const setupAdminSpy = vi.spyOn(auth, 'setupAdmin').mockResolvedValue({})
    const user = userEvent.setup()

    render(
      <MemoryRouter>
        <SetupScreen />
      </MemoryRouter>,
    )

    await user.type(screen.getByLabelText('Admin Username'), 'admin')
    await user.type(screen.getByLabelText('Email Address'), 'admin@example.com')
    await user.type(screen.getByLabelText(/^Password/), 'password123')
    await user.type(screen.getByLabelText('Confirm Password'), 'password123')

    await user.click(screen.getByRole('button', { name: 'Initialize Alexandryn' }))

    expect(setupAdminSpy).toHaveBeenCalledWith({
      username: 'admin',
      email: 'admin@example.com',
      password: 'password123',
    })
  })
})
