import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import 'antd/dist/reset.css'
import './styles/global.css'

import { AppRoot } from './App'

const container = document.getElementById('root')
if (!container) {
  throw new Error('Root container #root is missing from index.html')
}

/**
 * The window is an application, not a browser tab, so the WebView's own menu
 * (Back, Reload, Save as, Print, Inspect) has no business covering the app
 * surface. Menus the app draws itself — the explorer's right-click menus —
 * call `preventDefault` on their own and are unaffected by this.
 *
 * Text fields are the deliberate exception: there the native menu is the only
 * paste affordance, and Ctrl+C / Ctrl+V keep working everywhere either way.
 */
window.addEventListener('contextmenu', (event) => {
  const target = event.target as HTMLElement | null
  if (target?.closest('input, textarea, [contenteditable="true"], .cm-content')) return
  event.preventDefault()
})

createRoot(container).render(
  <StrictMode>
    <AppRoot />
  </StrictMode>,
)
