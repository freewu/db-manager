#!/usr/bin/env bash
#
# Assemble the directory that .github/workflows/pages.yml publishes.
#
#   bash scripts/pages-build.sh [output]      # default: _site
#
# `docs/` is the page as the repository keeps it, and there `../asserts/icon/…`
# and `../wails.json` are right: the page is served from the repository root (or
# opened from disk), so those files are one level up. A GitHub project site
# lives under a sub-path instead, where `../` climbs past the site root and
# 404s — so the published copy carries the engine artwork and the version
# manifest next to the page and drops the `../`. (The logo does not need this:
# `docs/logo.png` is the page's own copy of it.)
#
# `node scripts/docs-check.mjs --site _site` checks the result; CI runs both.

set -euo pipefail

cd "$(dirname "$0")/.."
out="${1:-_site}"

rm -rf "$out"
mkdir -p "$out"
cp -r docs/. "$out/"
cp -r asserts "$out/asserts"
cp wails.json "$out/wails.json"

# `sed -i` takes no argument on GNU sed and an empty one on BSD sed.
if sed --version >/dev/null 2>&1; then
  sedi() { sed -i "$@"; }
else
  sedi() { sed -i '' "$@"; }
fi
sedi 's|\.\./asserts/|asserts/|g' "$out/index.html"
sedi 's|\.\./wails\.json|wails.json|g' "$out/site.js"

echo "pages: $out — $(find "$out" -type f | wc -l) files"
