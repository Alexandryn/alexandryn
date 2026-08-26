import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { StatusPill } from './StatusPill'

describe('StatusPill', () => {
  it('always renders visible text, never a color-only signal', () => {
    render(<StatusPill tone="success">Available</StatusPill>)
    expect(screen.getByText('Available')).toBeInTheDocument()
  })

  it('applies a distinct token class per tone', () => {
    const tones = ['neutral', 'success', 'warning', 'error', 'accent'] as const
    const classNames = tones.map((tone) => {
      const { container, unmount } = render(<StatusPill tone={tone}>Status</StatusPill>)
      const className = container.querySelector('span')?.className
      unmount()
      return className
    })
    expect(new Set(classNames).size).toBe(tones.length)
  })

  it('has zero axe violations', async () => {
    const { container } = render(<StatusPill tone="error">Overdue</StatusPill>)
    expectNoAxeViolations(await runAxe(container))
  })
})
