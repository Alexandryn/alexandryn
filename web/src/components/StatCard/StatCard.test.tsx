import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { StatCard } from './StatCard'

describe('StatCard', () => {
  it('renders the label before the value in DOM order', () => {
    render(<StatCard label="Books read" value={42} />)
    const label = screen.getByText('Books read')
    const value = screen.getByText('42')
    expect(label.compareDocumentPosition(value) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('renders an optional hint', () => {
    render(<StatCard label="Books read" value={42} hint="This year" />)
    expect(screen.getByText('This year')).toBeInTheDocument()
  })

  it('has zero axe violations', async () => {
    const { container } = render(<StatCard label="Books read" value={42} />)
    expectNoAxeViolations(await runAxe(container))
  })
})
