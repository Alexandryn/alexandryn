import { contextBridge, ipcRenderer } from 'electron'
import { OPERATIONS, type AlexandrynDesktopBridge } from '../shared/operations'

// Iterate OPERATIONS array directly to construct the namespaced bridge object.
// One channel per operation, never a wildcard channel.

const bridge: Record<string, Record<string, (args?: unknown) => Promise<unknown>>> = {}

for (const op of OPERATIONS) {
  if (!bridge[op.namespace]) {
    bridge[op.namespace] = {}
  }
  bridge[op.namespace]![op.method] = (args?: unknown) => ipcRenderer.invoke(op.name, args)
}

contextBridge.exposeInMainWorld('alexandryn', bridge as unknown as AlexandrynDesktopBridge)
