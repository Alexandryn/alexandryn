import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { runAxe, expectNoAxeViolations } from '../../test/axe'
import { VisuallyHidden } from './VisuallyHidden'

describe('VisuallyHidden', () => {
  it('is present in the DOM and reachable by accessible-name queries', () => {
    render(
      <button>
        <VisuallyHidden>Close</VisuallyHidden>
      </button>,
    )
    expect(screen.getByRole('button', { name: 'Close' })).toBeInTheDocument()
  })

  it('is clipped from sighted view, never display:none or width:0 alone', () => {
    render(<VisuallyHidden>Hidden text</VisuallyHidden>)
    const node = screen.getByText('Hidden text')
    expect(node).toHaveStyle({ position: 'absolute', clip: 'rect(0px, 0px, 0px, 0px)' })
    expect(node).not.toHaveStyle({ display: 'none' })
  })

  it('has zero axe violations', async () => {
    const { container } = render(
      <button>
        <VisuallyHidden>Close</VisuallyHidden>
      </button>,
    )
    expectNoAxeViolations(await runAxe(container))
  })
})
