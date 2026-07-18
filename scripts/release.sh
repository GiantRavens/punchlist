#!/usr/bin/env bash
set -euo pipefail

# release.sh — publish the version in VERSION as a GitHub release and bump
# the Homebrew tap formula so `brew update && brew upgrade punchlist` picks
# it up everywhere.
#
# The full release flow is:
#   1. commit your work on main
#   2. bump VERSION (and add a CHANGELOG section)
#   3. run scripts/release.sh
#
# brew installs from the *tagged release tarball* pinned in the tap formula
# (url + sha256), not from the repo's main branch — pushing code alone never
# reaches brew users. This script does the tag, the GitHub release, and the
# formula bump in one shot.
#
#   --dry-run   print every action without tagging, releasing, or pushing.

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"

dry_run=false
[[ "${1:-}" == "--dry-run" ]] && dry_run=true

version="$(tr -d '[:space:]' < VERSION)"
tag="v${version}"
tarball_url="https://github.com/GiantRavens/punchlist/archive/refs/tags/${tag}.tar.gz"

run() { if $dry_run; then echo "DRY-RUN: $*"; else "$@"; fi; }

# --- preflight -------------------------------------------------------------
[[ -z "$(git status --porcelain)" ]] || { echo "error: working tree not clean — commit first" >&2; exit 1; }
[[ "$(git branch --show-current)" == "main" ]] || { echo "error: releases cut from main only" >&2; exit 1; }
if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
  echo "error: tag ${tag} already exists — bump VERSION first" >&2
  exit 1
fi
command -v gh >/dev/null || { echo "error: gh CLI required" >&2; exit 1; }
echo "running tests..."
go test ./... >/dev/null || { echo "error: tests failing — not releasing" >&2; exit 1; }

echo "releasing punchlist ${tag}"

# --- tag + GitHub release --------------------------------------------------
run git push origin main
run git tag -a "$tag" -m "punchlist ${version}"
run git push origin "$tag"
run gh release create "$tag" --title "punchlist ${version}" --generate-notes

# --- checksum the release tarball -----------------------------------------
if $dry_run; then
  sha="<sha256-of-${tag}-tarball>"
else
  sha="$(curl -fsSL --retry 5 --retry-delay 3 "$tarball_url" | shasum -a 256 | awk '{print $1}')"
  [[ -n "$sha" ]] || { echo "error: could not checksum ${tarball_url}" >&2; exit 1; }
fi

# --- bump the tap formula --------------------------------------------------
# prefer brew's own clone of the tap (a real pushable git checkout);
# fall back to a fresh temp clone when brew or the tap is absent (Linux).
tap_dir=""
tmp_parent=""
if command -v brew >/dev/null 2>&1; then
  candidate="$(brew --repository)/Library/Taps/giantravens/homebrew-tap"
  [[ -d "$candidate" ]] && tap_dir="$candidate"
fi
if [[ -z "$tap_dir" ]]; then
  tmp_parent="$(mktemp -d)"
  tap_dir="${tmp_parent}/homebrew-tap"
  run git clone git@github.com:GiantRavens/homebrew-tap.git "$tap_dir"
fi

formula="${tap_dir}/Formula/punchlist.rb"
if $dry_run; then
  echo "DRY-RUN: update ${formula}"
  echo "DRY-RUN:   url    -> ${tarball_url}"
  echo "DRY-RUN:   sha256 -> ${sha}"
  echo "DRY-RUN: git commit + push in ${tap_dir}"
else
  (
    cd "$tap_dir"
    git pull --ff-only
    perl -pi -e "s|url \".*\"|url \"${tarball_url}\"|; s|sha256 \".*\"|sha256 \"${sha}\"|" "$formula"
    git add "$formula"
    git commit -m "punchlist ${version}"
    git push
  )
fi
[[ -n "$tmp_parent" ]] && rm -rf "$tmp_parent"

echo
echo "released punchlist ${tag}"
echo "users (and you) update with: brew update && brew upgrade punchlist"
