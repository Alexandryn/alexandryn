import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { SettingsIndex } from './SettingsIndex'
import { MoreScreen } from '../shell/MoreScreen'

const hrefs = () =>
  screen.getAllByRole('link').map((a) => a.getAttribute('href'))

// Settings and More navigation index screens.
describe('Settings and More index screens', () => {
  it('/settings links to its real sub-pages', () => {
    render(
      <MemoryRouter>
        <SettingsIndex />
      </MemoryRouter>,
    )
    expect(hrefs()).toEqual(
      expect.arrayContaining(['/settings/network', '/settings/devices', '/libraries']),
    )
  })

  it('/more surfaces the screens the mobile tab bar has no room for', () => {
    render(
      <MemoryRouter>
        <MoreScreen />
      </MemoryRouter>,
    )
    expect(hrefs()).toEqual(
      expect.arrayContaining([
        '/activity',
        '/import',
        '/sources',
        '/libraries',
        '/settings/network',
        '/settings/devices',
      ]),
    )
  })
})
