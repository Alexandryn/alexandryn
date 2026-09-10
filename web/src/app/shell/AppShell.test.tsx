import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { act } from 'react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { mockMatchMedia } from '../../test/matchMedia'
import { AppShell } from './AppShell'

let media: ReturnType<typeof mockMatchMedia>

// Default: at/above the reflow breakpoint (the sidebar layout).
beforeEach(() => {
  media = mockMatchMedia(true)
})
afterEach(() => media.restore())

function routerAt(path: string) {
  return createMemoryRouter(
    [
      {
        path: '/',
        element: <AppShell />,
        children: [
          { path: 'library', element: <h1>Library</h1> },
          { path: 'discover', element: <h1>Discover</h1> },
        ],
      },
    ],
    { initialEntries: [path] },
  )
}

describe('AppShell', () => {
  it('exposes the three shell landmarks as semantic elements', () => {
    render(<RouterProvider router={routerAt('/library')} />)
    expect(screen.getByRole('banner')).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument()
    expect(screen.getByRole('main')).toBeInTheDocument()
  })

  it('renders a real search input that navigates to discover on submit (audit 0016 #236)', async () => {
    const router = routerAt('/library')
    render(<RouterProvider router={router} />)

    const searchInput = screen.getByRole('searchbox', {
      name: 'Search library, authors, subjects, ISBN',
    })
    expect(searchInput).toBeInTheDocument()

    await userEvent.type(searchInput, 'tolstoy{Enter}')
    expect(router.state.location.pathname).toBe('/discover')
    expect(router.state.location.search).toBe('?q=tolstoy')
  })

  it('marks the active route in the sidebar', () => {
    render(<RouterProvider router={routerAt('/discover')} />)
    expect(screen.getByRole('link', { name: 'Discover' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Library' })).not.toHaveAttribute('aria-current')
  })

  it('does not remount the shell frame when the route changes', async () => {
    const router = routerAt('/library')
    const { container } = render(<RouterProvider router={router} />)

    const headerBefore = container.querySelector('header')
    const mainBefore = container.querySelector('main')
    expect(await screen.findByRole('heading', { name: 'Library' })).toBeInTheDocument()

    await router.navigate('/discover')
    expect(await screen.findByRole('heading', { name: 'Discover' })).toBeInTheDocument()

    expect(container.querySelector('header')).toBe(headerBefore)
    expect(container.querySelector('main')).toBe(mainBefore)
  })

  it('offers a skip link that targets the content region', () => {
    render(<RouterProvider router={routerAt('/library')} />)
    expect(screen.getByRole('link', { name: 'Skip to content' })).toHaveAttribute('href', '#main')
    expect(screen.getByRole('main')).toHaveAttribute('id', 'main')
  })

  it('moves focus to the content region on navigation, but not on initial load', async () => {
    const router = routerAt('/library')
    render(<RouterProvider router={router} />)
    await screen.findByRole('heading', { name: 'Library' })
    expect(screen.getByRole('main')).not.toHaveFocus()

    await router.navigate('/discover')
    await screen.findByRole('heading', { name: 'Discover' })
    expect(screen.getByRole('main')).toHaveFocus()
  })

  it('has no axe violations', async () => {
    const { container } = render(<RouterProvider router={routerAt('/library')} />)
    expect(await screen.findByRole('heading', { name: 'Library' })).toBeInTheDocument()
    expectNoAxeViolations(await runAxe(container))
  })

  describe('responsive reflow (FR-3)', () => {
    it('renders the sidebar, not the tab bar, at/above the breakpoint', () => {
      media = mockMatchMedia(true)
      render(<RouterProvider router={routerAt('/library')} />)
      expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument() // sidebar-only item
      expect(screen.queryByRole('link', { name: 'More' })).not.toBeInTheDocument() // tab-bar-only item
    })

    it('renders the tab bar, not the sidebar, below the breakpoint', () => {
      media = mockMatchMedia(false)
      render(<RouterProvider router={routerAt('/library')} />)
      expect(screen.getByRole('link', { name: 'More' })).toBeInTheDocument()
      expect(screen.queryByRole('link', { name: 'Settings' })).not.toBeInTheDocument()
    })

    it('swaps sidebar↔tab bar when the viewport crosses the breakpoint', () => {
      media = mockMatchMedia(true)
      render(<RouterProvider router={routerAt('/library')} />)
      expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument()

      act(() => media.set(false)) // viewport shrinks past the breakpoint
      expect(screen.queryByRole('link', { name: 'Settings' })).not.toBeInTheDocument()
      expect(screen.getByRole('link', { name: 'More' })).toBeInTheDocument()

      act(() => media.set(true)) // and back
      expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument()
      expect(screen.queryByRole('link', { name: 'More' })).not.toBeInTheDocument()
    })

    it('keeps exactly one Primary nav landmark mounted in each layout', () => {
      media = mockMatchMedia(true)
      const { rerender } = render(<RouterProvider router={routerAt('/library')} />)
      expect(screen.getAllByRole('navigation', { name: 'Primary' })).toHaveLength(1)

      act(() => media.set(false))
      rerender(<RouterProvider router={routerAt('/library')} />)
      expect(screen.getAllByRole('navigation', { name: 'Primary' })).toHaveLength(1)
    })
  })
})
