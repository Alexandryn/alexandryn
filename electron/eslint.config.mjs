import js from '@eslint/js'
import globals from 'globals'
import tseslint from 'typescript-eslint'
import prettierConfig from 'eslint-config-prettier'

// Same core rule sets as web/eslint.config.js (js.recommended +
// typescript-eslint recommended + prettier), minus the React/browser
// plugins web needs, plus what the Electron privilege boundary needs.
// Shared formatting lives in the repo-root .prettierrc.json.
export default tseslint.config(
  { ignores: ['out'] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ['**/*.ts'],
    languageOptions: {
      ecmaVersion: 2023,
      // The main process and preload run in Node; the boot renderer runs
      // in Chromium. Both globals are permitted — each file uses only its
      // own, and the strict compiler options catch a genuine misuse.
      globals: { ...globals.node, ...globals.browser },
    },
    rules: {
      // architecture-desktop-host.md FR-1 / desktop-host-ipc-surface.md
      // FR-2: the preload exposes exactly one namespaced object built by
      // iterating electron/src/shared/operations.ts. A wildcard
      // passthrough — one channel taking an operation-name argument — is
      // the exact anti-pattern constitution §5 forbids. The structural
      // proof is a test (E23); this is the write-time nudge.
      'no-restricted-syntax': [
        'error',
        {
          selector:
            "CallExpression[callee.object.name='ipcRenderer'][callee.property.name=/^(on|once|send|sendSync|postMessage)$/]",
          message:
            'Use invoke/handle (request/response) per operation, never event-style ipcRenderer channels. See desktop-host-ipc-surface.md FR-2.',
        },
      ],
    },
  },
  prettierConfig,
)
