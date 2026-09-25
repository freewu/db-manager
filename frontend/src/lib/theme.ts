/**
 * The display theme: what the user asked for, and what that means right now.
 *
 * The preference has three settings — light, dark, and "follow the system" —
 * but only two of them can be painted, so every consumer reads the *resolved*
 * value. Keeping the resolution here (instead of in each component) is what
 * lets the operating system's switch flip the whole window at once, including
 * the CodeMirror editors and the ER diagram, which read the same value.
 */

import { t } from './i18n'
import type { MessageKey } from './i18n'

/** What the settings page stores. */
export type ThemeMode = 'light' | 'dark' | 'system'

/** What can actually be painted. */
export type ResolvedTheme = 'light' | 'dark'

/** The media query the OS answers: is the system theme dark? */
export const DARK_QUERY = '(prefers-color-scheme: dark)'

/**
 * What each setting is called, and what it means.
 *
 * The names live in the message tables rather than here, so that the settings
 * page can show them in the interface's language — which is read at the moment
 * the control is drawn, not at the moment this module is loaded.
 */
const THEME_WORDS: Record<ThemeMode, { labelKey: MessageKey; hintKey: MessageKey }> = {
  light: { labelKey: 'libTheme.light', hintKey: 'libTheme.light-hint' },
  dark: { labelKey: 'libTheme.dark', hintKey: 'libTheme.dark-hint' },
  system: { labelKey: 'libTheme.system', hintKey: 'libTheme.system-hint' },
}

/** The three settings, in the order they are offered. */
export const THEME_MODES: { value: ThemeMode; labelKey: MessageKey; hintKey: MessageKey }[] = (
  ['light', 'dark', 'system'] as ThemeMode[]
).map((value) => ({ value, ...THEME_WORDS[value] }))

/** What a setting is called in the interface. */
export function themeLabel(mode: ThemeMode): string {
  return t(THEME_WORDS[mode].labelKey)
}

/** One line about what a setting does, for a tooltip. */
export function themeHint(mode: ThemeMode): string {
  return t(THEME_WORDS[mode].hintKey)
}

/**
 * Whether the operating system is in dark mode.
 *
 * Guarded because the app also runs in a plain browser during frontend-only
 * development, where `matchMedia` may be missing or the query unsupported;
 * light is the answer in both cases, matching the default preference.
 */
export function systemPrefersDark(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
  try {
    return window.matchMedia(DARK_QUERY).matches
  } catch {
    return false
  }
}

/** The theme to paint for a preference. */
export function resolveTheme(mode: ThemeMode, systemDark = systemPrefersDark()): ResolvedTheme {
  if (mode === 'system') return systemDark ? 'dark' : 'light'
  return mode
}

/**
 * Reads a stored preference back. Anything unrecognised — an older build only
 * wrote `light` and `dark`, and the file can be edited by hand — falls back to
 * light, which is what Navicat's classic look is.
 */
export function parseThemeMode(value: unknown): ThemeMode {
  return value === 'dark' || value === 'system' ? value : 'light'
}

/**
 * Calls `onChange` whenever the system theme changes.
 *
 * `addEventListener` is the modern spelling; `addListener` is kept for older
 * webviews, which is exactly what a desktop app has to expect.
 */
export function watchSystemTheme(onChange: (dark: boolean) => void): () => void {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return () => undefined
  }
  const query = window.matchMedia(DARK_QUERY)
  const handler = (event: MediaQueryListEvent) => onChange(event.matches)
  if (typeof query.addEventListener === 'function') {
    query.addEventListener('change', handler)
    return () => query.removeEventListener('change', handler)
  }
  query.addListener(handler)
  return () => query.removeListener(handler)
}
