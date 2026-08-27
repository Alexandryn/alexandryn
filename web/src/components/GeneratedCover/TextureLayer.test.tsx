import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { deriveSeed } from '../../lib/fnv1a'
import { TextureLayer } from './TextureLayer'

describe('TextureLayer', () => {
  it('is hidden from the accessibility tree (decorative)', () => {
    const { container } = render(<TextureLayer seed={deriveSeed('work-1')} />)
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true')
  })

  it('renders each pattern variant without crashing', () => {
    for (const pattern of ['flat', 'diagonal-stripe', 'dot-grid'] as const) {
      const { container, unmount } = render(<TextureLayer seed={{ hue: 200, pattern }} />)
      expect(container.firstChild).toBeInTheDocument()
      unmount()
    }
  })

  it('renders identical style for the same seed', () => {
    const seed = deriveSeed('work-1')
    const a = render(<TextureLayer seed={seed} />)
    const b = render(<TextureLayer seed={seed} />)
    expect((a.container.firstChild as HTMLElement).getAttribute('style')).toBe(
      (b.container.firstChild as HTMLElement).getAttribute('style'),
    )
  })

  it('renders different style for a different hue', () => {
    const a = render(<TextureLayer seed={{ hue: 10, pattern: 'flat' }} />)
    const b = render(<TextureLayer seed={{ hue: 250, pattern: 'flat' }} />)
    expect((a.container.firstChild as HTMLElement).getAttribute('style')).not.toBe(
      (b.container.firstChild as HTMLElement).getAttribute('style'),
    )
  })
})
