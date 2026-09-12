import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { deriveSeed } from '../../lib/fnv1a'
import { SpineLayer } from './SpineLayer'

describe('SpineLayer', () => {
  it('is hidden from the accessibility tree (decorative)', () => {
    const { container } = render(<SpineLayer seed={deriveSeed('work-1')} />)
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true')
  })

  it('renders identical style for the same seed', () => {
    const seed = deriveSeed('work-1')
    const a = render(<SpineLayer seed={seed} />)
    const b = render(<SpineLayer seed={seed} />)
    expect((a.container.firstChild as HTMLElement).getAttribute('style')).toBe(
      (b.container.firstChild as HTMLElement).getAttribute('style'),
    )
  })

  it('renders different style for a different hue', () => {
    const a = render(<SpineLayer seed={{ hue: 10, pattern: 'flat' }} />)
    const b = render(<SpineLayer seed={{ hue: 250, pattern: 'flat' }} />)
    expect((a.container.firstChild as HTMLElement).getAttribute('style')).not.toBe(
      (b.container.firstChild as HTMLElement).getAttribute('style'),
    )
  })

  it('never lets a caller-supplied style override the seed-derived background (determinism)', () => {
    const seed = { hue: 10, pattern: 'flat' } as const
    const baseline = render(<SpineLayer seed={seed} />)
    const overridden = render(<SpineLayer seed={seed} style={{ backgroundColor: 'red' }} />)
    const overriddenNode = overridden.container.firstChild as HTMLElement
    expect(overriddenNode.style.backgroundColor).not.toBe('red')
    expect(overriddenNode.style.backgroundColor).toBe(
      (baseline.container.firstChild as HTMLElement).style.backgroundColor,
    )
  })

  it('stays hidden from the accessibility tree even if a caller passes a conflicting rest prop', () => {
    const { container } = render(
      <SpineLayer
        seed={deriveSeed('work-1')}
        {...({ 'aria-hidden': 'false' } as Record<string, string>)}
      />,
    )
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true')
  })
})
