import { render, screen } from '@testing-library/react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { Skeleton } from './Skeleton'

describe('Skeleton', () => {
  it('announces the loading state via role="status"', () => {
    render(<Skeleton label="Loading book cover" />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading book cover')
  })

  it('respects prefers-reduced-motion by disabling the shimmer', () => {
    render(<Skeleton />)
    const shape = screen.getByRole('status').querySelector('[aria-hidden="true"]')
    expect(shape).toHaveClass('animate-pulse', 'motion-reduce:animate-none')
  })

  it('accepts a custom className to size the placeholder shape', () => {
    render(<Skeleton className="h-3xl w-3xl" />)
    expect(screen.getByRole('status')).toHaveClass('h-3xl', 'w-3xl')
  })

  it('has zero axe violations', async () => {
    const { container } = render(<Skeleton />)
    expectNoAxeViolations(await runAxe(container))
  })

  it('defers the announced text to after mount rather than the initial render, so a live-region-watching screen reader sees a real mutation', () => {
    // SSR never runs effects, so this proves the label is genuinely
    // deferred (via useAnnouncedText), not just present synchronously.
    const html = renderToStaticMarkup(<Skeleton label="Loading book cover" />)
    expect(html).not.toContain('Loading book cover')
  })
})
