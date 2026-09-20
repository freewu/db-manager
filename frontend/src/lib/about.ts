/**
 * Project metadata for the welcome pane and *Help → About*.
 *
 * The versions are read from `package.json` rather than typed here, so a badge
 * can never advertise a version the bundle does not actually contain. The
 * author block mirrors the `author` entry of `wails.json` (which Wails consumes
 * for the native packaging metadata) — keep the two in step when it changes.
 */
import pkg from '../../package.json'

/** Where the source lives; also the base of every link the UI shows. */
export const PROJECT_URL = 'https://github.com/freewu/db-manager'

export const REPO_ISSUES_URL = `${PROJECT_URL}/issues`
export const REPO_RELEASES_URL = `${PROJECT_URL}/releases`

export const DEVELOPER = {
  name: 'bluefrog',
  email: 'bluefrog.wu@gmail.com',
}

export const LICENSE = 'MIT'

/**
 * Opens a URL in the user's browser.
 *
 * Wails injects `window.runtime` into the webview; under a plain browser (the
 * dev server, the CDP harness) the same call degrades to a new tab, which keeps
 * the UI usable outside the desktop shell.
 */
export function openExternal(url: string) {
  const runtime = (window as unknown as { runtime?: { BrowserOpenURL?: (url: string) => void } })
    .runtime
  if (runtime?.BrowserOpenURL) runtime.BrowserOpenURL(url)
  else window.open(url, '_blank', 'noopener')
}

/** One shields.io-style badge: a grey label and a coloured value. */
export interface Shield {
  label: string
  value: string
  /** Brand colour of the value half. */
  color: string
  /** Set for light brand colours that need dark text (React's cyan). */
  dark?: boolean
}

export interface ShieldGroup {
  title: string
  shields: Shield[]
}

const deps: Record<string, string> = {
  ...(pkg.dependencies as Record<string, string>),
  ...(pkg.devDependencies as Record<string, string>),
}

/** `"^6.6.4"` → `"6.6.4"`; a range specifier would only be noise on a badge. */
function depVersion(name: string): string {
  return (deps[name] ?? '').replace(/^[\s^~>=<v]+/, '')
}

/**
 * The stack, grouped the way it is layered.
 *
 * `goVersion` comes from the backend (`runtime.Version()` of the running
 * binary) — the one version only the binary itself can report.
 */
export function techStack(goVersion?: string): ShieldGroup[] {
  return [
    {
      title: 'Runtime',
      shields: [
        { label: 'go', value: goVersion || 'unknown', color: '#00ADD8' },
        { label: 'typescript', value: depVersion('typescript'), color: '#3178C6' },
      ],
    },
    {
      title: 'Desktop and UI',
      shields: [
        { label: 'wails', value: 'v2', color: '#DF0000' },
        { label: 'react', value: depVersion('react'), color: '#61DAFB', dark: true },
        { label: 'ant design', value: depVersion('antd'), color: '#1677FF' },
        { label: 'vite', value: depVersion('vite'), color: '#646CFF' },
        { label: 'codemirror', value: '6', color: '#4B5563' },
        { label: 'zustand', value: depVersion('zustand'), color: '#4B5563' },
      ],
    },
  ]
}
