import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The bundle is embedded into the Go binary by Wails (//go:embed
// frontend/dist). Relative asset URLs are used so the same build works from
// the dev server, from the embedded asset server and from a plain http server.
export default defineConfig({
  plugins: [react()],
  base: './',
  resolve: {
    alias: {
      // Brand artwork lives in the repository-level `asserts` folder so the
      // desktop packaging (build/appicon.png) and the webview share one source.
      '@asserts': fileURLToPath(new URL('../asserts', import.meta.url)),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // WebView2 is Chromium-based; Safari 15 covers the macOS WKWebView floor.
    target: ['chrome105', 'safari15'],
    sourcemap: false,
    chunkSizeWarningLimit: 1600,
  },
  server: {
    port: 34115,
    strictPort: false,
    // `@asserts` points outside the frontend package, so the dev server has to
    // be allowed to read one level up.
    fs: { allow: [fileURLToPath(new URL('..', import.meta.url))] },
  },
})
