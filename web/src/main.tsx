import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
// theme.css (generated, frontend-design-tokens.md) loads first so its
// Tailwind @theme tokens are available to every utility class; index.css
// is the pre-Tier-4 scaffold's own demo styling, replaced wholesale once
// Tier 4 builds the real shell.
import './theme.css'
import './index.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
