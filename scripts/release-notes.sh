#!/usr/bin/env bash
#
# Render the GitHub Release body for a tag.
#
#   scripts/release-notes.sh v0.2.0 > RELEASE_NOTES.md
#
# The body has three parts:
#   1. the annotated tag message, i.e. the hand written summary of the release,
#   2. the commits since the previous tag, grouped by Conventional Commit type,
#   3. the download table plus the per platform run instructions.
#
# Only git and coreutils are used, so it runs on every GitHub runner and can be
# previewed locally before the tag exists (it then falls back to HEAD).
set -euo pipefail

self=$(basename "$0")

tag="${1:-$(git describe --tags --abbrev=0 2>/dev/null || true)}"
if [ -z "$tag" ]; then
  echo "usage: ${self} <tag>" >&2
  exit 2
fi

version="${tag#v}"
tag_exists=false
if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
  tag_exists=true
fi

# Commits to list: everything since the previous tag, or the whole history for
# the very first release.
previous=""
if [ "$tag_exists" = true ]; then
  if previous=$(git describe --tags --abbrev=0 "${tag}^" 2>/dev/null); then
    range="${previous}..${tag}"
  else
    range="${tag}"
  fi
else
  echo "${self}: tag ${tag} does not exist yet, previewing HEAD" >&2
  previous=$(git describe --tags --abbrev=0 2>/dev/null || true)
  if [ -n "$previous" ]; then range="${previous}..HEAD"; else range="HEAD"; fi
fi

# The annotated tag message is the release summary — `just publish` stores the
# hand written summary of the version there, and writes the placeholder
# `Release v1.2.3` when none was given, which would only repeat the release
# title, so treat it as empty. Paragraphs are preserved, leading and trailing
# blank lines are not.
#
# The ref has to be an annotated *tag* object. On a lightweight tag `%(contents)`
# resolves straight to the commit and returns the **commit message**, which would
# silently replace the summary with the body of the last commit. (GitHub's
# actions/checkout leaves such a lightweight ref behind, which is why the release
# workflow fetches the tag object again before calling this script.)
summary=""
if [ "$tag_exists" = true ]; then
  case "$(git cat-file -t "refs/tags/${tag}" 2>/dev/null || echo missing)" in
    tag)
      summary=$(git tag -l --format='%(contents)' "$tag" |
        awk 'BEGIN { RS = "" } { out = out (NR > 1 ? "\n\n" : "") $0 } END { print out }')
      if [ "$summary" = "Release v${version}" ]; then
        summary=""
      fi
      ;;
    *)
      echo "${self}: ${tag} is not an annotated tag in this clone, so there is no release summary" >&2
      ;;
  esac
fi

subjects=$(mktemp)
trap 'rm -f "$subjects"' EXIT
git log --no-merges --pretty=tformat:'%s' "$range" \
  | grep -v -E '^chore\(release\)' >"$subjects" || true

added=""
fixed=""
performance=""
refactored=""
documented=""
tested=""
other=""

append() { # $1 = variable name, $2 = commit subject
  printf -v "$1" '%s- %s\n' "${!1}" "$2"
}

while IFS= read -r subject; do
  [ -n "$subject" ] || continue
  case "$subject" in
    feat*) append added "$subject" ;;
    fix*) append fixed "$subject" ;;
    perf*) append performance "$subject" ;;
    refactor*) append refactored "$subject" ;;
    docs*) append documented "$subject" ;;
    test*) append tested "$subject" ;;
    *) append other "$subject" ;;
  esac
done <"$subjects"

section() { # $1 = heading, $2 = body
  if [ -n "$2" ]; then
    printf '### %s\n\n%s\n' "$1" "$2"
  fi
}

slug="${GITHUB_REPOSITORY:-}"
if [ -z "$slug" ]; then
  slug=$(git remote get-url origin 2>/dev/null |
    sed -e 's#^git@github.com:##' -e 's#^https://github.com/##' -e 's#\.git$##') || true
fi

if [ -n "$summary" ]; then
  printf '%s\n\n' "$summary"
fi

printf '## 本版内容\n\n'
if [ -s "$subjects" ]; then
  section "新增" "$added"
  section "修复" "$fixed"
  section "性能" "$performance"
  section "重构" "$refactored"
  section "文档" "$documented"
  section "测试" "$tested"
  section "其他" "$other"
else
  printf '首个版本。\n\n'
fi

if [ -n "$previous" ] && [ -n "$slug" ]; then
  printf '**完整变更**：https://github.com/%s/compare/%s...%s\n\n' "$slug" "$previous" "$tag"
fi

cat <<'EOF'
## 下载

| 平台 | 文件 | 运行方式 |
| --- | --- | --- |
| Windows x64 | `db-manager.exe` | 免安装，双击即可。需要 WebView2 运行时（Windows 10/11 自带） |
| macOS（Intel + Apple Silicon 通用） | `db-manager-macos-universal.zip` | 解压后把 `db-manager.app` 拖入「应用程序」 |
| Linux x64 | `db-manager` | `chmod +x db-manager && ./db-manager` |

前端资源已经编译进可执行文件，无需另外安装 Node.js / Go，也不依赖任何运行时目录。

macOS 包未做代码签名与公证，首次打开若被 Gatekeeper 拦下，请右键「打开」，或执行：

```sh
xattr -cr /Applications/db-manager.app
```

Linux 需要 GTK3 与 WebKitGTK 4.0（Debian/Ubuntu：`libgtk-3-0 libwebkit2gtk-4.0-37`）。

## 校验

下载后可比对本页附带的 `checksums.txt`：

```sh
sha256sum -c checksums.txt
```
EOF
