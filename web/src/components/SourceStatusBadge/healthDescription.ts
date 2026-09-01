import type { StatusTone } from '../StatusPill/StatusPill'
import type { SourceHealth } from '../../data/sources'

const DETAIL_TEXT: Record<string, string> = {
  timeout: "Can't connect — timed out",
  'connection-refused': "Can't connect — connection refused",
  'auth-rejected': 'Wrong username or password',
  'http-4xx': 'This source returned an error',
  'http-5xx': 'This source is having a problem right now',
  'http-3xx-unsupported': "This source tried to redirect, which isn't supported",
  'unparseable-response': 'Received an unexpected response',
  'path-not-found': 'Folder not found',
  'path-not-readable': "Folder isn't readable",
}

export function getHealthDescription(
  health: SourceHealth,
  isChecking?: boolean,
): { text: string; tone: StatusTone } {
  if (isChecking) {
    return { text: 'Checking...', tone: 'neutral' }
  }
  if (health.status === 'reachable') {
    return { text: 'Reachable', tone: 'success' }
  }
  if (health.status === 'unknown') {
    return { text: 'Checking...', tone: 'neutral' }
  }
  if (health.detail && DETAIL_TEXT[health.detail]) {
    return { text: DETAIL_TEXT[health.detail]!, tone: 'error' }
  }
  return { text: 'Unreachable', tone: 'error' }
}
