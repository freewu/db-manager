import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import 'antd/dist/reset.css'
import './styles/global.css'

import { AppRoot } from './App'
import { watchTraySettings } from './lib/tray'

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

/**
 * The tray menu (Windows only) can change the display mode and the language, and
 * reports a pick as an event. It is heard from here rather than from a component
 * so that a pick made in the first moments of the window's life is applied like
 * any other — the icon is in the notification area before the tree has mounted.
 */
watchTraySettings()

createRoot(container).render(
  <StrictMode>
    <AppRoot />
  </StrictMode>,
)
