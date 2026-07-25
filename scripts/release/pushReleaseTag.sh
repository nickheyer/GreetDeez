#!/usr/bin/env bash
# Cut next release, find the highest v*, bump it
set -euo pipefail

SKIP_PATHS=()

usage() { sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'; }

BUMP=patch
SET=''
DRY=false
while [ $# -gt 0 ]; do
  case "$1" in
    --major | major) BUMP=major ;;
    --minor | minor) BUMP=minor ;;
    --patch | patch) BUMP=patch ;;
    --set)
      SET="${2:?--set needs a version}"
      shift
      ;;
    --dry-run | -n) DRY=true ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      echo "pushReleaseTag: unknown argument '$1'" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

cd "$(git rev-parse --show-toplevel)"
git symbolic-ref -q HEAD > /dev/null || {
  echo "pushReleaseTag: detached HEAD - check out a branch first" >&2
  exit 1
}

# local tags can lag the remote after work from another machine
git fetch --tags --quiet origin || echo "warning: could not fetch tags from origin; using local tags" >&2

# highest existing app version
BASE=$(git tag --list 'v[0-9]*' | sed 's/^v//' | sort -V | tail -1)
BASE=${BASE:-0.0.0}
BASE=${BASE%%-*}
[[ $BASE =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || {
  echo "pushReleaseTag: highest tag v$BASE is not X.Y.Z" >&2
  exit 1
}

if [ -n "$SET" ]; then
  NEW=$SET
  [[ $NEW =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || {
    echo "pushReleaseTag: --set '$SET' is not X.Y.Z" >&2
    exit 1
  }
else
  IFS=. read -r MA MI PA <<< "$BASE"
  case "$BUMP" in
    major) NEW="$((MA + 1)).0.0" ;;
    minor) NEW="$MA.$((MI + 1)).0" ;;
    patch) NEW="$MA.$MI.$((PA + 1))" ;;
  esac
fi
TAG="v$NEW"

git rev-parse -q --verify "refs/tags/$TAG" > /dev/null && {
  echo "pushReleaseTag: $TAG already exists locally" >&2
  exit 1
}
[ -n "$(git ls-remote --tags origin "$TAG" 2> /dev/null)" ] && {
  echo "pushReleaseTag: $TAG already exists on origin" >&2
  exit 1
}

echo "pushReleaseTag: v$BASE -> $TAG"

skipped() {
  local f
  for f in "${SKIP_PATHS[@]:-}"; do [ "$f" = "$1" ] && return 0; done
  return 1
}

# ---- sync hardcoded versions -----------------------------------------------

mapfile -t PKGS < <(git ls-files -- 'package.json' '*/package.json')

TOUCHED=()
for f in "${PKGS[@]}"; do
  skipped "$f" && continue
  # only packages that version themselves - the UI package.jsons don't
  if ! grep -qE '^[[:space:]]*"version"[[:space:]]*:' "$f"; then continue; fi
  if $DRY; then
    echo "  would bump $f (npm version)"
    continue
  fi
  # npm keeps the file's own indentation and drags package-lock.json along
  (cd "$(dirname "$f")" \
    && npm version "$NEW" --no-git-tag-version --allow-same-version --ignore-scripts > /dev/null)
  TOUCHED+=("$f")
  [ -f "$(dirname "$f")/package-lock.json" ] && TOUCHED+=("$(dirname "$f")/package-lock.json")
done

# ---- commit, tag, push ------------------------------------------------------

if $DRY; then
  echo "  would commit as 'release: $TAG', tag $TAG, and push HEAD + tag to origin"
  ! git diff --cached --quiet && echo "  note: currently staged changes would ride the release commit"
  exit 0
fi

[ ${#TOUCHED[@]} -gt 0 ] && git add -- "${TOUCHED[@]}"

if ! git diff --cached --quiet; then
  # pre-staged work rides along: the tag ships HEAD either way, so leaving
  # it staged-but-uncommitted would only make the tag lie about the tree
  git commit -q -m "release: $TAG"
  echo "committed release: $TAG"
else
  echo "nothing to commit (versions already at $NEW); tagging HEAD as-is"
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "warning: unstaged/untracked changes left behind - they are NOT part of $TAG:" >&2
  git status --porcelain | sed 's/^/    /' >&2
fi

git tag -a "$TAG" -m "greetdeez $NEW"
# one push, two refs: a stale branch fails the whole push before the tag
# can escape and trigger release.yml
git push origin HEAD "refs/tags/$TAG"
echo "pushed $TAG - release.yml is building it"
