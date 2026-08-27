import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { GeneratedCover } from './GeneratedCover'

// frontend-generated-covers.md Accessibility: the cover is decorative once
// real title/author text exists elsewhere on the book's card — it must
// never be the sole accessible name, and its own rendered title/author
// pixels must never double up a surrounding element's accessible name.

describe('GeneratedCover — accessibility contract', () => {
  it('is hidden from the accessibility tree as a whole', () => {
    const { container } = render(
      <GeneratedCover identifier="work-1" title="Dune" author="Frank Herbert" />,
    )
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true')
  })

  it('is never the sole accessible name for its book — a real heading in the surrounding component provides it, not duplicated by the cover', () => {
    render(
      <a href="/books/1">
        <GeneratedCover identifier="work-1" title="Dune" author="Frank Herbert" />
        <h3>Dune</h3>
      </a>,
    )
    const link = screen.getByRole('link')
    expect(link).toHaveAccessibleName('Dune')
  })

  it('stays hidden from the accessibility tree even if a caller passes a conflicting rest prop', () => {
    // aria-hidden is excluded from GeneratedCoverProps at the type level;
    // this proves the runtime guard (aria-hidden applied after {...rest})
    // holds even if that type exclusion is ever bypassed (e.g. `as any`).
    const { container } = render(
      <GeneratedCover
        identifier="work-1"
        title="Dune"
        {...({ 'aria-hidden': 'false' } as Record<string, string>)}
      />,
    )
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true')
  })

  it('has zero axe violations when composed with a real accessible name alongside it', async () => {
    const { container } = render(
      <a href="/books/1">
        <GeneratedCover identifier="work-1" title="Dune" author="Frank Herbert" />
        <h3>Dune</h3>
      </a>,
    )
    expectNoAxeViolations(await runAxe(container))
  })
})
