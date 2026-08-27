import { getJson } from './http'

/**
 * Host-only capabilities (architecture-frontend.md FR-3's named set:
 * Settings, System, Sources configuration, Import). The real source of
 * this value is phase 12's concern; phase 04's mock grants all of them.
 */
export type Capability = 'sources' | 'import' | 'settings' | 'system'

export interface Bootstrap {
  capabilities: Record<Capability, boolean>
}

export function fetchBootstrap(): Promise<Bootstrap> {
  return getJson<Bootstrap>('/api/bootstrap')
}
