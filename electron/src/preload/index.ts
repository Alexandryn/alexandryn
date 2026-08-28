import { contextBridge } from 'electron'

// Phase 05 scaffold (E2). Tier 4 (E20) replaces this stub with the
// enumerated surface built by iterating electron/src/shared/operations.ts
// — one namespaced object, one channel per operation, every argument
// validated by Zod in the main process (desktop-host-ipc-surface.md
// FR-1/FR-2). For now the namespace exists but is empty.
contextBridge.exposeInMainWorld('alexandryn', {})
