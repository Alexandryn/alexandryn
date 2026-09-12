import { describe, expect, it } from 'vitest'
import { importPollInterval } from './import'

describe('importPollInterval', () => {
  it('stops polling entirely while the tab is hidden', () => {
    expect(importPollInterval(5, true)).toBe(false)
    expect(importPollInterval(0, true)).toBe(false)
  })

  it('polls fast while there are queued candidates', () => {
    expect(importPollInterval(1, false)).toBe(2000)
    expect(importPollInterval(12, false)).toBe(2000)
  })

  it('backs off to a slow floor when the queue is empty', () => {
    expect(importPollInterval(0, false)).toBe(20000)
  })
})
