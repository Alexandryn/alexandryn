import { getJson } from './http'

/**
 * Host-only capabilities (named set: Settings, System, Sources
 * configuration, Import, Network).
 */
export type Capability = 'sources' | 'import' | 'settings' | 'system' | 'network'

export interface Bootstrap {
  capabilities: Record<Capability, boolean>
}

export function fetchBootstrap(): Promise<Bootstrap> {
  return getJson<Bootstrap>('/api/bootstrap')
}
