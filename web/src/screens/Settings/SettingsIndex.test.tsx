import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it } from 'vitest'
import { SettingsIndex } from './SettingsIndex'
import { MoreScreen } from '../shell/MoreScreen'
import { clearSession, setCurrentUser } from '../../data/auth'

const hrefs = () =>
  screen.getAllByRole('link').map((a) => a.getAttribute('href'))

describe('Settings and More index screens — Role-Based Access Control', () => {
  beforeEach(() => {
    clearSession()
  })

  it('/settings for admin links to both Account Settings and Instance Administration', () => {
    setCurrentUser({
      id: 'admin-1',
      username: 'admin',
      email: 'admin@local',
      role: 'admin',
    })

    render(
      <MemoryRouter>
        <SettingsIndex />
      </MemoryRouter>,
    )

    const links = hrefs()
    // Account section
    expect(links).toContain('/settings/account')
    expect(links).toContain('/settings/devices')
    // Admin section
    expect(links).toContain('/settings/network')
    expect(links).toContain('/libraries')
    expect(links).toContain('/sources')
    expect(links).toContain('/activity')
    expect(links).toContain('/settings/diagnostics')

    expect(screen.getByRole('heading', { name: 'Account Settings' })).toBeDefined()
    expect(screen.getByRole('heading', { name: 'Instance Administration' })).toBeDefined()
  })

  it('/settings for regular reader links ONLY to Account Settings and hides Administration', () => {
    setCurrentUser({
      id: 'reader-1',
      username: 'reader',
      email: 'reader@local',
      role: 'reader',
    })

    render(
      <MemoryRouter>
        <SettingsIndex />
      </MemoryRouter>,
    )

    const links = hrefs()
    // Account section present
    expect(links).toContain('/settings/account')
    expect(links).toContain('/settings/devices')

    // Administrative sections strictly forbidden & hidden
    expect(links).not.toContain('/settings/network')
    expect(links).not.toContain('/libraries')
    expect(links).not.toContain('/sources')
    expect(links).not.toContain('/activity')
    expect(links).not.toContain('/settings/diagnostics')

    expect(screen.getByRole('heading', { name: 'Account Settings' })).toBeDefined()
    expect(screen.queryByRole('heading', { name: 'Instance Administration' })).toBeNull()
  })

  it('/more for admin surfaces both account and administrative sections', () => {
    setCurrentUser({
      id: 'admin-1',
      username: 'admin',
      email: 'admin@local',
      role: 'admin',
    })

    render(
      <MemoryRouter>
        <MoreScreen />
      </MemoryRouter>,
    )

    const links = hrefs()
    expect(links).toEqual(
      expect.arrayContaining([
        '/settings/account',
        '/settings/devices',
        '/import',
        '/sources',
        '/libraries',
        '/activity',
        '/settings/network',
        '/settings/diagnostics',
      ]),
    )
  })

  it('/more for regular reader hides administrative navigation', () => {
    setCurrentUser({
      id: 'reader-1',
      username: 'reader',
      email: 'reader@local',
      role: 'reader',
    })

    render(
      <MemoryRouter>
        <MoreScreen />
      </MemoryRouter>,
    )

    const links = hrefs()
    expect(links).toContain('/settings/account')
    expect(links).toContain('/settings/devices')

    // Administrative destinations are not leaked
    expect(links).not.toContain('/sources')
    expect(links).not.toContain('/libraries')
    expect(links).not.toContain('/activity')
    expect(links).not.toContain('/settings/network')
    expect(links).not.toContain('/settings/diagnostics')
  })
})
