import { render, screen } from '@testing-library/react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { Spinner } from './Spinner'

describe('Spinner', () => {
  it('announces the loading state via role="status"', () => {
    render(<Spinner label="Loading collection" />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading collection')
  })

  it('defaults to a generic "Loading" label', () => {
    render(<Spinner />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading')
  })

  it('respects prefers-reduced-motion by disabling the spin animation', () => {
    render(<Spinner />)
    const icon = screen.getByRole('status').querySelector('[aria-hidden="true"]')
    expect(icon).toHaveClass('animate-spin', 'motion-reduce:animate-none')
  })

  it('has zero axe violations', async () => {
    const { container } = render(<Spinner />)
    expectNoAxeViolations(await runAxe(container))
  })

  it('defers the announced text to after mount rather than the initial render, so a live-region-watching screen reader sees a real mutation', () => {
    // SSR never runs effects, so this proves the label is genuinely
    // deferred (via useAnnouncedText), not just present synchronously —
    // a real screen-reader bug: content already there when the role="status"
    // region is first inserted is not guaranteed to be announced.
    const html = renderToStaticMarkup(<Spinner label="Loading collection" />)
    expect(html).not.toContain('Loading collection')
  })
})
