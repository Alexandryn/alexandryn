import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
// theme.css (generated, frontend-design-tokens.md) carries the @theme
// tokens every utility class resolves against.
import './theme.css'
import { AppRoot } from './app/AppRoot'

// MSW (frontend-shell-and-routing.md FR-6) runs only in development. The
// dynamic import behind a statically-false `import.meta.env.DEV` is
// dead-code-eliminated from the production build, so neither ./mocks/browser
// nor the "mockServiceWorker" string reaches web/dist (check:dist-msw).
async function enableMocking(): Promise<void> {
  if (!import.meta.env.DEV) return
  const { worker } = await import('./mocks/browser')
  await worker.start({ onUnhandledRequest: 'bypass' })
}

void enableMocking().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <AppRoot />
    </StrictMode>,
  )
})
