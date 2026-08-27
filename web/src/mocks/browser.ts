import { setupWorker } from 'msw/browser'
import { handlers } from './handlers'

// Used only in development and by the Playwright E2E harness — imported
// dynamically behind `import.meta.env.DEV` in main.tsx so neither this
// module nor MSW's worker-script name reaches a production bundle
// (frontend-shell-and-routing.md Security considerations; check:dist-msw).
export const worker = setupWorker(...handlers)
