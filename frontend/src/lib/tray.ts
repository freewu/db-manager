/**
 * The tray menu's two settings, and how a pick in it reaches the interface.
 *
 * The menu itself is native (see tray.go): the backend draws it, ticks what is in
 * force, and reports a pick over the Wails event bus. The preferences, though, are
 * this side's — the store owns them and the store is what writes them — so a pick
 * is applied exactly like a pick in the settings page, and the tray's ticks are
 * read back out of what was stored. Nothing here writes a preference itself: two
 * writers of one state file is how a theme change ends up dropping the language.
 *
 * The two event names are a contract with the Go side — `trayThemeEvent` and
 * `trayLanguageEvent` in tray.go, whose spelling is pinned by
 * TestTrayEventsAreTheNamesTheFrontendListensFor.
 */

import { runtime } from '../api/client'
import { useAppStore } from '../store/appStore'
import { parseLanguage } from './i18n'
import { parseThemeMode } from './theme'

/** The display-mode rows of the tray menu. */
export const TRAY_THEME_EVENT = 'tray:theme'

/** The language rows of the tray menu. */
export const TRAY_LANGUAGE_EVENT = 'tray:language'

/**
 * Lets the tray menu change the display mode and the language, for as long as the
 * window lives.
 *
 * Installed from `main.tsx` rather than from a component: the menu is there as
 * soon as the app is — before any component has mounted — and an event with
 * nobody listening is gone. (A pick made in the moment between the icon appearing
 * and this module running is still lost, since nothing can hear it yet.) Returns
 * the unsubscribe, which is what a test or a remount needs.
 *
 * Both values come back as strings and go through the same parsers the stored
 * ones do, so a value from a build that offered something this one does not is
 * ignored instead of half applied.
 */
export function watchTraySettings(): () => void {
  const offTheme = runtime.onEvent(TRAY_THEME_EVENT, (mode) => {
    useAppStore.getState().setTheme(parseThemeMode(mode))
  })
  const offLanguage = runtime.onEvent(TRAY_LANGUAGE_EVENT, (language) => {
    useAppStore.getState().setUiLanguage(parseLanguage(language))
  })
  return () => {
    offTheme()
    offLanguage()
  }
}
