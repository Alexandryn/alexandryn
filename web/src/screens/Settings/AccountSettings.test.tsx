import { fireEvent, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { clearSession, setCurrentUser } from '../../data/auth'
import { renderWithProviders } from '../../test/renderWithProviders'
import { AccountSettings } from './AccountSettings'

describe('AccountSettings Screen', () => {
  beforeEach(() => {
    clearSession()
  })

  it('renders account details for administrator', () => {
    setCurrentUser({
      id: 'admin-1',
      username: 'alex_admin',
      email: 'alex@library.lan',
      role: 'admin',
    })

    renderWithProviders(<AccountSettings />, { routerEntries: ['/settings/account'] })

    expect(screen.getByText('alex_admin')).toBeInTheDocument()
    expect(screen.getByText('alex@library.lan')).toBeInTheDocument()
    expect(screen.getByText('Administrator')).toBeInTheDocument()

    // Connected devices shortcut
    const devicesLink = screen.getByRole('link', { name: /view and manage connected devices/i })
    expect(devicesLink).toBeInTheDocument()
    expect(devicesLink).toHaveAttribute('href', '/settings/devices')
  })

  it('renders account details for regular reader', () => {
    setCurrentUser({
      id: 'reader-1',
      username: 'book_lover',
      email: 'reader@library.lan',
      role: 'reader',
    })

    renderWithProviders(<AccountSettings />, { routerEntries: ['/settings/account'] })

    expect(screen.getByText('book_lover')).toBeInTheDocument()
    expect(screen.getByText('reader@library.lan')).toBeInTheDocument()
    expect(screen.getByText('Reader')).toBeInTheDocument()
  })

  it('opens MFA setup modal when configure button is clicked', async () => {
    setCurrentUser({
      id: 'user-1',
      username: 'user',
      email: 'user@library.lan',
      role: 'reader',
    })

    renderWithProviders(<AccountSettings />, { routerEntries: ['/settings/account'] })

    const configureBtn = screen.getByRole('button', { name: /configure authenticator app/i })
    expect(configureBtn).toBeInTheDocument()

    fireEvent.click(configureBtn)

    // Modal opens
    expect(await screen.findByText('Set up two-factor authentication')).toBeInTheDocument()
  })
})
