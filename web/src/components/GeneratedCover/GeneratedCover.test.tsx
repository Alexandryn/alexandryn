import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { GeneratedCover } from './GeneratedCover'

describe('GeneratedCover — degradation ladder (FR-3)', () => {
  it('step 1: title + author present renders the full four-layer composition', () => {
    render(<GeneratedCover identifier="work-1" title="Dune" author="Frank Herbert" />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.getByText('Frank Herbert')).toBeInTheDocument()
  })

  it('step 1: title layout sits at the bottom when author is present', () => {
    render(<GeneratedCover identifier="work-1" title="Dune" author="Frank Herbert" />)
    const textOverlay = screen.getByText('Dune').parentElement
    expect(textOverlay).toHaveClass('justify-end')
  })

  it('step 2: title present, author absent renders title only, re-centered', () => {
    render(<GeneratedCover identifier="work-1" title="Dune" />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.queryByText('Frank Herbert')).not.toBeInTheDocument()
    expect(screen.getByText('Dune').parentElement).toHaveClass('justify-center')
  })

  it('step 3: neither title nor author renders no text layer, never a blank box', () => {
    const { container } = render(<GeneratedCover identifier="work-1" />)
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()
    // Still a "cover shape" — texture + spine always present.
    expect(container.firstChild?.childNodes.length).toBeGreaterThanOrEqual(2)
  })

  it('every step renders the texture and spine layers (never a blank box)', () => {
    const cases = [{ title: 'Dune', author: 'Frank Herbert' }, { title: 'Dune' }, {}]
    for (const props of cases) {
      const { container, unmount } = render(<GeneratedCover identifier="work-1" {...props} />)
      const layers = container.querySelectorAll('[aria-hidden="true"]')
      expect(layers.length).toBeGreaterThanOrEqual(2)
      unmount()
    }
  })
})
