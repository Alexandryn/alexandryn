import { render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { AppShell } from './AppShell'

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
    expect(screen.getByRole('banner')).toBeInTheDocument() // <header>
    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument()
    expect(screen.getByRole('main')).toBeInTheDocument()
  })

  it('marks the active route in the sidebar', () => {
    render(<RouterProvider router={routerAt('/discover')} />)
    const active = screen.getByRole('link', { name: 'Discover' })
    expect(active).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Library' })).not.toHaveAttribute('aria-current')
  })

  it('does not remount the shell frame when the route changes', async () => {
    const router = routerAt('/library')
    const { container } = render(<RouterProvider router={router} />)

    const headerBefore = container.querySelector('header')
    const navBefore = container.querySelector('nav')
    expect(await screen.findByRole('heading', { name: 'Library' })).toBeInTheDocument()

    await router.navigate('/discover')
    expect(await screen.findByRole('heading', { name: 'Discover' })).toBeInTheDocument()

    // Same DOM nodes: the frame was never torn down and rebuilt.
    expect(container.querySelector('header')).toBe(headerBefore)
    expect(container.querySelector('nav')).toBe(navBefore)
  })

  it('offers a skip link that targets the content region', () => {
    render(<RouterProvider router={routerAt('/library')} />)
    expect(screen.getByRole('link', { name: 'Skip to content' })).toHaveAttribute('href', '#main')
    expect(screen.getByRole('main')).toHaveAttribute('id', 'main')
  })

  it('has no axe violations', async () => {
    const { container } = render(<RouterProvider router={routerAt('/library')} />)
    expect(await screen.findByRole('heading', { name: 'Library' })).toBeInTheDocument()
    expectNoAxeViolations(await runAxe(container))
  })
})
