#!/usr/bin/env node
/**
 * Single source of truth for the product version.
 *
 * The canonical value lives in `wails.json` (`info.productVersion`); every other
 * file that carries a version mirrors it, so cutting a release is one command:
 *
 *   node scripts/version.mjs 0.2.0     # sync every mirror
 *   node scripts/version.mjs --check   # fail if the mirrors drifted apart
 *   node scripts/version.mjs --get     # print the current version
 *
 * `just publish 0.2.0` wraps the first form, so you normally never call this by
 * hand. Mirrors are rewritten with anchored regular expressions instead of
 * re-serialising JSON, which keeps the diffs to a single line per file.
 */
import { readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

const SEMVER = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/

/**
 * `app.go` is the fallback used by `wails dev` and by builds without ldflags.
 * The `-dev` suffix keeps it distinguishable from a released binary, whose
 * version is injected by `wails build -ldflags "-X main.Version=…"`.
 */
const DEV_SUFFIX = '-dev'

const MIRRORS = [
  {
    file: 'wails.json',
    describe: 'canonical version',
    pattern: /("productVersion"\s*:\s*")[^"]*(")/,
    value: (version) => version,
  },
  {
    file: 'frontend/package.json',
    describe: 'frontend package',
    pattern: /(^[ \t]*"version"\s*:\s*")[^"]*(")/m,
    value: (version) => version,
  },
  {
    file: 'Justfile',
    describe: '`just build` ldflags',
    pattern: /(^version[ \t]*:=[ \t]*")[^"]*(")/m,
    value: (version) => version,
  },
  {
    file: 'app.go',
    describe: 'development fallback',
    pattern: /(^var Version = ")[^"]*(")/m,
    value: (version) => version + DEV_SUFFIX,
  },
]

const read = (file) => readFileSync(path.join(root, file), 'utf8')

/** The version currently recorded in every mirror. */
function readMirror(mirror) {
  const match = read(mirror.file).match(mirror.pattern)
  if (!match) {
    throw new Error(`${mirror.file}: cannot find the version (expected ${mirror.pattern})`)
  }
  return match[0].slice(match[1].length, match[0].length - match[2].length)
}

function currentVersion() {
  return readMirror(MIRRORS[0])
}

/** Rewrite every mirror to `version`; returns the files that actually changed. */
function writeVersion(version) {
  const changed = []
  for (const mirror of MIRRORS) {
    const source = read(mirror.file)
    const next = source.replace(mirror.pattern, `$1${mirror.value(version)}$2`)
    if (next === source) continue
    writeFileSync(path.join(root, mirror.file), next)
    changed.push(`${mirror.file} (${mirror.describe}) -> ${mirror.value(version)}`)
  }
  return changed
}

function checkMirrors(expected) {
  const canonical = currentVersion()
  const problems = []
  if (expected && canonical !== expected) {
    problems.push(`wails.json is ${canonical} but the tag says ${expected}`)
  }
  for (const mirror of MIRRORS.slice(1)) {
    const actual = readMirror(mirror)
    const want = mirror.value(expected ?? canonical)
    if (actual !== want) {
      problems.push(`${mirror.file} is ${actual} but should be ${want} (${mirror.describe})`)
    }
  }
  return { canonical, problems }
}

const USAGE = `usage: node scripts/version.mjs [<x.y.z> | --get | --check [<x.y.z>]]

  <x.y.z>          write the version into every file that carries it
  --get            print the current version
  --check [<x.y.z>]  verify the mirrors agree, and optionally that they match
                     <x.y.z> (used by the release workflow to guard the tag)`

const [command, ...rest] = process.argv.slice(2)

if (command === undefined || command === '-h' || command === '--help') {
  console.log(USAGE)
  process.exit(command === undefined ? 2 : 0)
}

if (command === '--get') {
  console.log(currentVersion())
} else if (command === '--check') {
  const expected = rest[0]
  if (expected && !SEMVER.test(expected)) {
    console.error(`version.mjs: "${expected}" is not a semver version`)
    process.exit(2)
  }
  const { canonical, problems } = checkMirrors(expected)
  if (problems.length > 0) {
    console.error('version.mjs: version mismatch:')
    for (const problem of problems) console.error(`  - ${problem}`)
    console.error('  Run `just publish <version>` (or `node scripts/version.mjs <version>`) to fix it.')
    process.exit(1)
  }
  console.log(`version.mjs: every mirror is at ${canonical}`)
} else {
  if (!SEMVER.test(command)) {
    console.error(`version.mjs: "${command}" is not a semver version`)
    console.error(USAGE)
    process.exit(2)
  }
  const previous = currentVersion()
  const changed = writeVersion(command)
  if (changed.length === 0) {
    console.log(`version.mjs: already at ${command}, nothing to do`)
  } else {
    console.log(`version.mjs: ${previous} -> ${command}`)
    for (const line of changed) console.log(`  - ${line}`)
  }
}
