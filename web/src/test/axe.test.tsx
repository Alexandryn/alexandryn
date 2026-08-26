import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { expectNoAxeViolations, runAxe } from './axe'

describe('runAxe', () => {
  it('flags an unlabelled input', async () => {
    const { container } = render(<input type="text" />)
    const violations = await runAxe(container)
    expect(violations.some((v) => v.id === 'label')).toBe(true)
    expect(() => expectNoAxeViolations(violations)).toThrow(/label/)
  })

  it('passes a correctly labelled input', async () => {
    const { container } = render(
      <label>
        Name
        <input type="text" />
      </label>,
    )
    const violations = await runAxe(container)
    expect(() => expectNoAxeViolations(violations)).not.toThrow()
  })
})
