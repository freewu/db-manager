#!/usr/bin/env node
/**
 * Archive the `wails build` output into a locally distributable package.
 *
 *   node scripts/package.mjs
 *
 * Only the host platform is packaged, because Wails cannot cross-compile macOS
 * from anywhere else and a local package is meant to be used right here.
 * Windows and Linux get the bare executable (it is already standalone — the
 * frontend bundle is embedded), macOS gets a `.tar.gz` because a `.app` is a
 * directory. A `checksums.txt` is written next to the artefacts.
 *
 * Nothing here touches git: releasing to GitHub is `just publish`.
 */
import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

const OS_NAMES = { win32: 'windows', darwin: 'macos', linux: 'linux' }
const ARCH_NAMES = { x64: 'amd64', arm64: 'arm64', ia32: '386' }

const os = OS_NAMES[process.platform] ?? process.platform
const arch = ARCH_NAMES[process.arch] ?? process.arch
const exeSuffix = process.platform === 'win32' ? '.exe' : ''
const flavour = `${os}-${arch}`

const fail = (message) => {
  console.error(`package.mjs: ${message}`)
  process.exit(1)
}

const version = (() => {
  const manifest = JSON.parse(readFileSync(path.join(root, 'wails.json'), 'utf8'))
  const value = manifest?.info?.productVersion
  if (!value) fail('wails.json: no info.productVersion')
  return value
})()

const binDir = path.join(root, 'build', 'bin')
const distDir = path.join(root, 'dist')

/** sha256 of a file, hex encoded — the same format `sha256sum -c` expects. */
function sha256(file) {
  return createHash('sha256').update(readFileSync(file)).digest('hex')
}

const humanSize = (file) => `${(statSync(file).size / 1024 / 1024).toFixed(1)} MB`

rmSync(distDir, { recursive: true, force: true })
mkdirSync(distDir, { recursive: true })

const artefacts = []

if (process.platform === 'darwin') {
  const bundle = 'db-manager.app'
  if (!existsSync(path.join(binDir, bundle))) {
    fail(`build/bin/${bundle} is missing — run \`just release\`, not this script, on its own`)
  }
  const target = path.join(distDir, `db-manager-${version}-${flavour}.tar.gz`)
  // macOS ships bsdtar, and so does Windows 10+; no npm dependency needed.
  const tar = spawnSync('tar', ['-czf', target, '-C', binDir, bundle], { stdio: 'inherit' })
  if (tar.status !== 0) fail('tar failed')
  artefacts.push(target)
} else {
  const source = path.join(binDir, `db-manager${exeSuffix}`)
  if (!existsSync(source)) {
    fail(`build/bin/db-manager${exeSuffix} is missing — run \`just release\`, not this script, on its own`)
  }
  const target = path.join(distDir, `db-manager-${version}-${flavour}${exeSuffix}`)
  cpSync(source, target)
  artefacts.push(target)
}

const checksumLines = artefacts.map((file) => `${sha256(file)}  ${path.basename(file)}`)
writeFileSync(path.join(distDir, 'checksums.txt'), `${checksumLines.join('\n')}\n`)

console.log(`\npackage.mjs: db-manager ${version} for ${flavour}`)
for (const file of artefacts) {
  console.log(`  dist/${path.basename(file)}  (${humanSize(file)})`)
}
console.log('  dist/checksums.txt')
