import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { FormatBadge } from './FormatBadge'

describe('FormatBadge', () => {
  it('renders the format label as text', () => {
    render(<FormatBadge format="EPUB" />)
    expect(screen.getByText('EPUB')).toBeInTheDocument()
  })

  it('uses the monospace token font, not an ad hoc font stack', () => {
    render(<FormatBadge format="PDF" />)
    expect(screen.getByText('PDF')).toHaveClass('font-mono')
  })

  it('has zero axe violations', async () => {
    const { container } = render(<FormatBadge format="EPUB" />)
    expectNoAxeViolations(await runAxe(container))
  })
})
