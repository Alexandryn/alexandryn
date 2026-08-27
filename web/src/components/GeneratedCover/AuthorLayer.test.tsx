import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { AuthorLayer } from './AuthorLayer'

describe('AuthorLayer', () => {
  it('renders the given display string as-is', () => {
    render(<AuthorLayer author="Frank Herbert" />)
    expect(screen.getByText('Frank Herbert')).toBeInTheDocument()
  })

  it('renders an already-reduced multi-author string without reducing it further', () => {
    render(<AuthorLayer author="Frank Herbert et al." />)
    expect(screen.getByText('Frank Herbert et al.')).toBeInTheDocument()
  })

  it('is smaller than the title layer by token, not by ad hoc value', () => {
    render(<AuthorLayer author="Frank Herbert" />)
    expect(screen.getByText('Frank Herbert')).toHaveClass('text-xs')
  })
})
