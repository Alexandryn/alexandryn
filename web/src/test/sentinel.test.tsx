import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

// frontend-tooling.md FR-7's own acceptance criterion: prove the Vitest +
// React Testing Library runner works, not test any real component — this
// file has no other purpose and is not meant to survive once Tier 2's
// primitives exist.
function Sentinel() {
  return <p>sentinel</p>
}

describe('Vitest + React Testing Library runner', () => {
  it('renders a component and finds it in the DOM', () => {
    render(<Sentinel />)
    expect(screen.getByText('sentinel')).toBeInTheDocument()
  })
})
