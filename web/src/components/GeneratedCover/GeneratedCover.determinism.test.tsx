import { render, cleanup } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { GeneratedCover } from './GeneratedCover'

// Determinism end to end through the whole composed render pipeline
// already proves the hash function itself is deterministic in isolation;
// this proves the pipeline built on top of it (layers + ladder) is too.

describe('GeneratedCover — determinism', () => {
  it('renders pixel-identical output for the same identifier across two independent mounts', () => {
    const props = { identifier: 'work-42', title: 'Dune', author: 'Frank Herbert' }
    const a = render(<GeneratedCover {...props} />)
    const htmlA = a.container.innerHTML
    a.unmount()

    const b = render(<GeneratedCover {...props} />)
    const htmlB = b.container.innerHTML

    expect(htmlA).toBe(htmlB)
  })

  it('stays identical across a simulated fresh session (full unmount + remount, no shared React state)', () => {
    const props = { identifier: 'work-42', title: 'Dune', author: 'Frank Herbert' }
    const first = render(<GeneratedCover {...props} />)
    const htmlFirst = first.container.innerHTML
    cleanup() // tears down the whole tree — nothing survives to the next render

    const second = render(<GeneratedCover {...props} />)
    expect(second.container.innerHTML).toBe(htmlFirst)
  })

  it('renders different output for a different identifier', () => {
    const a = render(<GeneratedCover identifier="work-1" title="Dune" author="Frank Herbert" />)
    const htmlA = a.container.innerHTML
    a.unmount()

    const b = render(<GeneratedCover identifier="work-2" title="Dune" author="Frank Herbert" />)
    expect(b.container.innerHTML).not.toBe(htmlA)
  })

  it('is deterministic across every step of the degradation ladder, not just the full-data case', () => {
    for (const props of [
      { identifier: 'work-7', title: 'Dune', author: 'Frank Herbert' },
      { identifier: 'work-7', title: 'Dune' },
      { identifier: 'work-7' },
    ]) {
      const a = render(<GeneratedCover {...props} />)
      const htmlA = a.container.innerHTML
      a.unmount()

      const b = render(<GeneratedCover {...props} />)
      expect(b.container.innerHTML).toBe(htmlA)
      b.unmount()
    }
  })
})
