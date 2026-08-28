import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Gallery } from './Gallery'
import './gallery.css'

const container = document.getElementById('root')
if (!container) throw new Error('a11y gallery harness: #root not found')

createRoot(container).render(
  <StrictMode>
    <Gallery />
  </StrictMode>,
)
