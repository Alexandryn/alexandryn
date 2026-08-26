import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { ProgressBar } from './ProgressBar'

describe('ProgressBar', () => {
  it('renders a determinate bar with computed ARIA value state', () => {
    render(<ProgressBar label="Import progress" value={40} max={100} />)
    const bar = screen.getByRole('progressbar', { name: 'Import progress' })
    expect(bar).toHaveAttribute('value', '40')
    expect(bar).toHaveAttribute('max', '100')
  })

  it('renders an indeterminate bar with no value attribute', () => {
    render(<ProgressBar label="Loading" />)
    const bar = screen.getByRole('progressbar', { name: 'Loading' })
    expect(bar).not.toHaveAttribute('value')
  })

  it('respects prefers-reduced-motion by disabling the indeterminate animation', () => {
    render(<ProgressBar label="Loading" />)
    const bar = screen.getByRole('progressbar')
    expect(bar).toHaveClass('animate-pulse', 'motion-reduce:animate-none')
  })

  it('has zero axe violations in both determinate and indeterminate states', async () => {
    const { container, rerender } = render(<ProgressBar label="Import progress" value={40} />)
    expectNoAxeViolations(await runAxe(container))
    rerender(<ProgressBar label="Loading" />)
    expectNoAxeViolations(await runAxe(container))
  })
})
