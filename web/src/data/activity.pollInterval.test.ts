import { describe, expect, it } from 'vitest'
import { activityPollInterval, type SystemEvent } from './activity'

function ev(kind: string, jobId: string): SystemEvent {
  return {
    id: 1,
    event_kind: kind,
    job_id: jobId,
    payload: {},
    created_at: '2026-09-08T00:00:00Z',
    purge_at: '2026-10-08T00:00:00Z',
  }
}

describe('activityPollInterval (audit 0016 issue 102)', () => {
  it('stops entirely while the tab is hidden', () => {
    expect(activityPollInterval([ev('job.running', 'j1')], { active: true }, true)).toBe(false)
  })

  it('polls fast while a job is running', () => {
    expect(activityPollInterval([ev('job.running', 'j1')], {}, false)).toBe(5000)
  })

  it('falls back to a slow idle floor when nothing is active', () => {
    expect(activityPollInterval([ev('job.completed', 'j1')], {}, false)).toBe(60000)
    expect(activityPollInterval(undefined, {}, false)).toBe(60000)
  })

  it('polls fast when the Activity screen is open even if idle', () => {
    expect(activityPollInterval([], { active: true }, false)).toBe(5000)
  })
})
