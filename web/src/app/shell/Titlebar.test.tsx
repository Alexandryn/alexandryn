import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it } from 'vitest'
import { clearSession, setCurrentUser } from '../../data/auth'
import { Titlebar } from './Titlebar'

describe('Titlebar Role-Based Controls', () => {
  beforeEach(() => {
    clearSession()
  })

  it('renders hosting pill for administrator and links user badge to account settings', () => {
    setCurrentUser({
      id: 'admin-1',
      username: 'admin',
      email: 'admin@local',
      role: 'admin',
    })

    render(
      <MemoryRouter>
        <Titlebar />
      </MemoryRouter>,
    )

    // Hosting status pill visible for admin
    const hostingLink = screen.getByRole('link', { name: /hosting/i })
    expect(hostingLink).toBeInTheDocument()
    expect(hostingLink).toHaveAttribute('href', '/network')

    // User badge links to /settings/account with role title
    const userBadgeLink = screen.getByRole('link', { name: 'ad' })
    expect(userBadgeLink).toBeInTheDocument()
    expect(userBadgeLink).toHaveAttribute('href', '/settings/account')
    expect(userBadgeLink).toHaveAttribute('title', 'Account (Administrator): admin')

    // Sign out button is present
    expect(screen.getByRole('button', { name: /sign out/i })).toBeInTheDocument()
  })

  it('hides hosting pill for regular reader but preserves account link and sign out', () => {
    setCurrentUser({
      id: 'reader-1',
      username: 'reader',
      email: 'reader@local',
      role: 'reader',
    })

    render(
      <MemoryRouter>
        <Titlebar />
      </MemoryRouter>,
    )

    // Hosting status pill strictly hidden for regular reader
    expect(screen.queryByRole('link', { name: /hosting/i })).toBeNull()

    // User badge links to /settings/account with Reader title
    const userBadgeLink = screen.getByRole('link', { name: 're' })
    expect(userBadgeLink).toBeInTheDocument()
    expect(userBadgeLink).toHaveAttribute('href', '/settings/account')
    expect(userBadgeLink).toHaveAttribute('title', 'Account (Reader): reader')

    // Sign out button is present
    expect(screen.getByRole('button', { name: /sign out/i })).toBeInTheDocument()
  })
})
