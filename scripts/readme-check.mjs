#!/usr/bin/env node
//
// Check the three READMEs.
//
//   node scripts/readme-check.mjs
//
// README.md is the English document; README_CN.md and README_TC.md are its
// mirrors, and the download table also has to agree with the asset names the
// release workflow produces. Nothing else would notice a mirror that fell
// behind, a relative link that rotted, or a renamed release asset, so this
// script is that notice: it fails, with the list of problems, in CI and before
// a commit.
//
// Only the Node standard library is used, so it runs anywhere `node` does.

import { readFileSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join, resolve } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')

const MIRRORS = ['README.md', 'README_CN.md', 'README_TC.md']
const SOURCE = MIRRORS[0]

const problems = []
const read = (path) => readFileSync(join(root, path), 'utf8')

/* ------------------------------------------------------------- structure */

/**
 * The translations carry different prose, so only the *shape* is compared:
 * the heading levels in order, fenced blocks, table lines and top-level list
 * items. A section that was dropped, a table row that went missing or a code
 * fence that was never closed shows up as a mismatch here.
 */
function shape(text) {
  return {
    headings: [...text.matchAll(/^(#{1,6}) .*$/gm)].map((m) => m[1].length).join(','),
    fences: [...text.matchAll(/^```/gm)].length,
    tables: [...text.matchAll(/^\|/gm)].length,
    bullets: [...text.matchAll(/^- /gm)].length,
  }
}

const shapes = new Map()
for (const name of MIRRORS) {
  if (!existsSync(join(root, name))) {
    problems.push(`${name} is missing`)
    continue
  }
  shapes.set(name, shape(read(name)))
}

const source = shapes.get(SOURCE)
for (const [name, other] of shapes) {
  if (name === SOURCE || !source) continue
  for (const [part, value] of Object.entries(other)) {
    if (value !== source[part]) {
      problems.push(`${name}: ${part} differ from ${SOURCE} (${value} vs ${source[part]})`)
    }
  }
}

/* --------------------------------------------------------------- mirrors */

// Each mirror has to point at the other two, or a reader who lands on the
// Chinese page cannot find the English one.
for (const name of MIRRORS) {
  if (!shapes.has(name)) continue
  const text = read(name)
  for (const other of MIRRORS) {
    if (other === name) continue
    if (!text.includes(`](${other})`)) problems.push(`${name} does not link to ${other}`)
  }
}

/* ----------------------------------------------------------------- links */

// A relative link is a promise that the file is still there. Anchors, absolute
// URLs and mail addresses are none of our business.
const links = new Set()
for (const name of MIRRORS) {
  if (!shapes.has(name)) continue
  for (const match of read(name).matchAll(/\]\(([^)]+)\)/g)) {
    const url = match[1]
    if (/^(https?:|#|mailto:)/.test(url)) continue
    links.add(`${name}\u0000${url.split('#')[0]}`)
  }
}

for (const entry of links) {
  const [name, url] = entry.split('\u0000')
  if (!url) continue
  if (!existsSync(join(root, url))) problems.push(`${name} links to ${url}, which does not exist`)
}

/* -------------------------------------------------------- release assets */

// The asset names are written down twice — `release.yml` renames the build
// output, `release-notes.sh` prints the download table — and the READMEs are
// the third copy. This is the string that keeps all three honest.
const workflow = read('.github/workflows/release.yml')
const suffixes = [...workflow.matchAll(/^\s+suffix: (.+)$/gm)].map((m) => m[1].trim())
if (suffixes.length === 0) problems.push('release.yml declares no asset suffixes')

const notes = read('scripts/release-notes.sh')
for (const suffix of suffixes) {
  // The download tables keep the name in the second cell; a rename that only
  // touched the "how to run" cell next to it would otherwise slip through.
  const row = new RegExp(`^\\|[^|]*\\|\\s*\`db-manager-<version>-${suffix.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\``, 'm')
  for (const name of MIRRORS) {
    if (!shapes.has(name)) continue
    if (!row.test(read(name))) problems.push(`${name} does not list "${suffix}" as a download`)
  }
  if (!notes.includes(suffix)) problems.push(`release-notes.sh does not mention the "${suffix}" download`)
}

/* ---------------------------------------------------------------- report */

if (problems.length) {
  console.error(`readme: ${problems.length} problem(s)\n`)
  for (const problem of problems) console.error(`  - ${problem}`)
  process.exit(1)
}

const sections = source.headings.split(',').length
console.log(
  `readme: ok — ${MIRRORS.length} mirrors, ${sections} sections, ` +
    `${source.tables} table lines, ${suffixes.length} downloads`,
)
