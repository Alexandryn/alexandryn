import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { CoverGridHarness } from './CoverGridHarness'
import '../../src/theme.css'

const identifiers = Array.from({ length: 500 }, (_, i) => `bench-work-${i}`)

performance.mark('cover-grid:data-ready')

const container = document.getElementById('root')
if (!container) throw new Error('benchmark harness: #root not found')

createRoot(container).render(
  <StrictMode>
    <CoverGridHarness
      identifiers={identifiers}
      onRendered={() => {
        // Double rAF: the first fires before the browser has necessarily
        // painted the committed DOM, the second is guaranteed to run after
        // that paint — so "viewport-rendered" reflects real paint, not just
        // React's commit.
        requestAnimationFrame(() => {
          requestAnimationFrame(() => {
            performance.mark('cover-grid:viewport-rendered')
            performance.measure(
              'cover-grid:initial-viewport',
              'cover-grid:data-ready',
              'cover-grid:viewport-rendered',
            )
          })
        })
      }}
    />
  </StrictMode>,
)
