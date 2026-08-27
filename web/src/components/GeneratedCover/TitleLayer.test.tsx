import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { TitleLayer } from './TitleLayer'

describe('TitleLayer', () => {
  it('renders the title text', () => {
    render(<TitleLayer title="Dune" />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
  })

  it('uses the Newsreader typography token', () => {
    render(<TitleLayer title="Dune" />)
    expect(screen.getByText('Dune')).toHaveClass('font-reading')
  })

  it('clamps rather than overflows a long title', () => {
    const long =
      'A Title So Long It Would Otherwise Overflow The Cover Layout Entirely Without Truncation'
    render(<TitleLayer title={long} />)
    const node = screen.getByText(long)
    expect(node).toHaveClass('line-clamp-3')
  })
})
