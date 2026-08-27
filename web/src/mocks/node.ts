import { setupServer } from 'msw/node'
import { handlers } from './handlers'

// Used by the Vitest setup file (src/test/setup.ts) — the network-layer
// interception boundary for unit and integration tests.
export const server = setupServer(...handlers)
