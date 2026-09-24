import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { AlexAvatar } from './AlexAvatar'
import { AlexMascot } from './AlexMascot'

describe('AlexAvatar', () => {
  it('renders decorative circular badge without axe violations', async () => {
    const { container } = render(<AlexAvatar />)
    expect(container.querySelector('svg')).toBeInTheDocument()
    expectNoAxeViolations(await runAxe(container))
  })
})

describe('AlexMascot', () => {
  it('renders with default reading mood', () => {
    render(<AlexMascot />)
    const mascot = screen.getByTestId('alex-mascot')
    expect(mascot).toHaveAttribute('data-mood', 'reading')
  })

  it('renders with custom moods', () => {
    const { rerender } = render(<AlexMascot mood="sleeping" />)
    expect(screen.getByTestId('alex-mascot')).toHaveAttribute('data-mood', 'sleeping')

    rerender(<AlexMascot mood="searching" />)
    expect(screen.getByTestId('alex-mascot')).toHaveAttribute('data-mood', 'searching')

    rerender(<AlexMascot mood="celebrating" />)
    expect(screen.getByTestId('alex-mascot')).toHaveAttribute('data-mood', 'celebrating')
  })

  it('has zero axe violations', async () => {
    const { container } = render(<AlexMascot mood="reading" />)
    expectNoAxeViolations(await runAxe(container))
  })
})
