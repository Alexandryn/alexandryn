import { render, screen } from '@testing-library/react'
import { act } from 'react'
import { afterEach, describe, expect, it } from 'vitest'
import { mockMatchMedia } from '../test/matchMedia'
import { useMediaQuery } from './useMediaQuery'

let control: ReturnType<typeof mockMatchMedia> | undefined
afterEach(() => control?.restore())

function Probe() {
  return <p>{useMediaQuery('(min-width: 768px)') ? 'wide' : 'narrow'}</p>
}

describe('useMediaQuery', () => {
  it('reflects the initial match state', () => {
    control = mockMatchMedia(true)
    render(<Probe />)
    expect(screen.getByText('wide')).toBeInTheDocument()
  })

  it('re-renders when the query flips', () => {
    control = mockMatchMedia(false)
    render(<Probe />)
    expect(screen.getByText('narrow')).toBeInTheDocument()

    act(() => control!.set(true))
    expect(screen.getByText('wide')).toBeInTheDocument()
  })
})
