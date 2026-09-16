# DB Manager — developer tasks.
#
# Recipes are written so they run from a POSIX shell (WSL, macOS, Linux) and
# from PowerShell on Windows. The few that cannot be expressed portably come
# in two variants, separated by the `[unix]` / `[windows]` attributes.
#
# `wails` builds a desktop binary, so on Windows everything must run with the
# Windows toolchain: run `just` from Windows (or `just.exe`), not from WSL.

set windows-shell := ["powershell.exe", "-NoLogo", "-Command"]
set shell := ["sh", "-cu"]

web := "frontend"
bin := "build/bin"
exe := "db-manager"
version := "0.1.0"

# List the available recipes.
default:
    @just --list

# ------------------------------------------------------------------ setup ---

# Install the npm packages and download the Go modules.
install:
    npm --prefix {{web}} install
    go mod download

# Tidy the module graph and vet every package.
tidy:
    go mod tidy
    go vet ./...

# ---------------------------------------------------------------- quality ---

# Type check the frontend (this is also what validates the Ant Design API).
typecheck:
    npm --prefix {{web}} run typecheck

# Run the Go test suite.
test:
    go test ./...

# ------------------------------------------------------------------ build ---

# Build the frontend bundle on its own (no Go involved).
frontend-build: icons
    npm --prefix {{web}} run build

# Build the Windows desktop binary, production frontend bundle included.
build: icons
    wails build -clean -platform windows/amd64 -ldflags "-X main.Version={{version}}"

# Same as `build` but without the clean step, for quick iterations.
build-fast: icons
    wails build -platform windows/amd64

# Package the app for this machine: build, then archive the result into dist/
# next to a checksum file. Purely local — it never touches git, so it is safe to
# run on a dirty tree. Releasing to GitHub is `just publish`.
#
# The version stamped into the binary comes from wails.json; run
# `node scripts/version.mjs <x.y.z>` first if you want it to match something.
#
# Package this machine's build into dist/ (no git involved).
release: icons
    wails build -clean -ldflags "-X main.Version={{version}}"
    node scripts/package.mjs

# Run the app in development mode: Vite dev server + Go live reload.
dev: icons
    wails dev

# Regenerate the Wails JS/TS bindings into ./frontend/wailsjs.
generate:
    wails generate module

# Launch the previously built binary.
[unix]
run:
    ./{{bin}}/{{exe}}

[windows]
run:
    & "{{bin}}/{{exe}}.exe"

# ------------------------------------------------------------------ build ---

# `asserts/logo.png` is the single source of truth for the brand artwork:
# `frontend/public/logo.png` (webview favicon) and `build/appicon.png` (desktop
# icon) are copies, and the cached `windows/icon.ico` is deleted so that Wails
# rebuilds it from the new PNG.
#
# Sync brand artwork from asserts/ into the build.
[unix]
icons:
    cp asserts/logo.png {{web}}/public/logo.png
    cp asserts/logo.png build/appicon.png
    rm -f build/windows/icon.ico

[windows]
icons:
    Copy-Item -Force asserts/logo.png {{web}}/public/logo.png
    Copy-Item -Force asserts/logo.png build/appicon.png
    Remove-Item -Force -ErrorAction SilentlyContinue build/windows/icon.ico

# ---------------------------------------------------------------- utility ---

# Print the version of every tool the build depends on.
doctor:
    go version
    node --version
    npm --version
    wails version

# Remove every build artefact.
[unix]
clean:
    rm -rf {{bin}} {{web}}/dist dist

[windows]
clean:
    if (Test-Path {{bin}}) { Remove-Item -Recurse -Force {{bin}} }
    if (Test-Path {{web}}/dist) { Remove-Item -Recurse -Force {{web}}/dist }
    if (Test-Path dist) { Remove-Item -Recurse -Force dist }

# ------------------------------------------------------- version / release ---

# Print the current version (single source of truth: wails.json).
ver:
    node scripts/version.mjs --get

# Fail when the version mirrors have drifted apart.
check-version:
    node scripts/version.mjs --check

# Preview the GitHub Release body for a tag (works before the tag exists).
notes tag:
    @bash scripts/release-notes.sh {{tag}}

# Write the message as a Conventional Commit — it becomes a bullet in the next
# release message. This is the mandatory end of every task, see AGENTS.md.
#
# Commit everything and push the current task.
sync message:
    git add -A
    git commit -m "{{message}}"
    git push origin HEAD

# Cut a release for GitHub: bump the version in every file that carries it,
# commit, tag and push. Pushing the tag runs .github/workflows/release.yml,
# which builds the standalone Windows / macOS / Linux executables and publishes
# the GitHub Release.
#
#   just publish 0.2.0 "新增 Navicat 风格连接树，索引成为一等资源"
#
# The optional summary becomes the head of the release message — it is stored in
# the annotated tag, so it travels with the tag and survives a re-run. Use
# `just release` instead if you only want a package for this machine.
#
# Bump, tag and push, triggering the GitHub Action release.
publish v summary='':
    node scripts/version.mjs {{v}}
    git add -A
    git commit -m "chore(release): v{{v}}"
    git tag -a v{{v}} -m "{{ if summary == '' { 'Release v' + v } else { summary } }}"
    git push origin HEAD
    git push origin v{{v}}
