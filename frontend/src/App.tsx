import { useEffect, useRef } from 'react'
import { App as AntApp, ConfigProvider, Result, Spin, theme as antdTheme } from 'antd'

import { AppShell } from './components/AppShell'
import { isDesktop } from './api/client'
import { useAppStore } from './store/appStore'
import { antdLocaleOf } from './lib/i18n/antd'
import { t, useLanguage } from './lib/i18n'

/**
 * Application root: resolves the persisted UI preferences, wires the Ant
 * Design theme and gates the shell behind the backend bootstrap.
 */
export function AppRoot() {
  const boot = useAppStore((s) => s.boot)
  const bootError = useAppStore((s) => s.bootError)
  // What is painted is the resolved theme, not the preference: `system` is a
  // question for the operating system, and the store has already asked it.
  const themeResolved = useAppStore((s) => s.resolvedTheme)
  const bootstrap = useAppStore((s) => s.bootstrap)
  // Switching the language re-renders everything below this component, which is
  // what makes an ordinary `t` call in a leaf component correct: the words are
  // read again because the leaf was drawn again.
  const language = useLanguage()

  // StrictMode runs effects twice in development; bootstrap must be idempotent.
  const started = useRef(false)
  useEffect(() => {
    if (started.current) return
    started.current = true
    void bootstrap()
  }, [bootstrap])

  useEffect(() => {
    document.documentElement.dataset.theme = themeResolved
    document.documentElement.style.colorScheme = themeResolved
  }, [themeResolved])

  const dark = themeResolved === 'dark'

  return (
    <ConfigProvider
      locale={antdLocaleOf(language)}
      theme={{
        algorithm: dark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          // Navicat is a dense 9pt UI: small type, square corners, flat chrome.
          fontSize: 12,
          borderRadius: 3,
          // Brand green. Keep in sync with `--dm-accent` in styles/global.css.
          colorPrimary: '#36ab60',
          colorBgLayout: dark ? '#101216' : '#f0f0f0',
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif',
        },
        components: {
          Table: { cellPaddingBlockSM: 2, cellPaddingInlineSM: 8 },
          Tree: { titleHeight: 22 },
          Tabs: { horizontalItemPadding: '4px 10px' },
          Layout: { bodyBg: dark ? '#101216' : '#f0f0f0' },
        },
      }}
    >
      <AntApp style={{ height: '100%' }}>
        {boot === 'loading' ? <BootSplash /> : null}
        {boot === 'failed' ? <BootFailure message={bootError} /> : null}
        {boot === 'ready' ? <AppShell /> : null}
      </AntApp>
    </ConfigProvider>
  )
}

function BootSplash() {
  return (
    <div
      style={{
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 16,
      }}
    >
      <Spin size="large" />
      <span style={{ opacity: 0.6 }}>{t('app.starting')}</span>
    </div>
  )
}

function BootFailure({ message }: { message?: string }) {
  const detail = isDesktop()
    ? (message ?? t('app.backend-failed'))
    : t('app.not-in-shell')

  return (
    <div
      style={{
        height: '100%',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <Result
        status="warning"
        title={t('app.cannot-reach-backend')}
        subTitle={<span className="mono">{detail}</span>}
      />
    </div>
  )
}
