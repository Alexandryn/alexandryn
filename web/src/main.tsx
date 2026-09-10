import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
// theme.css (generated, frontend-design-tokens.md) carries the @theme
// tokens every utility class resolves against; a11y.css layers the
// prefers-contrast adaptation on top (frontend-accessibility.md FR-5);
// utilities.css adds the hand-authored global classes that are not
// tokens.
import './theme.css'
import './a11y.css'
import './utilities.css'
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

function render(): void {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <AppRoot />
    </StrictMode>,
  )
}

// A mock-layer failure (dev only) must not leave a blank page — render the
// app regardless, having logged the problem.
enableMocking()
  .catch((error: unknown) => {
    console.error('MSW dev worker failed to start; continuing without mocks', error)
  })
  .finally(render)
