#!/usr/bin/env bash
# web-typecheck.sh — frontends/web's `tsc -b`, starting from the newest
# incremental state any checkout of this clone on this machine left.
#
#   scripts/web-typecheck.sh            (the merge gate's typecheck phase)
#
# tsc -b is incremental: its build info (node_modules/.tmp/*.tsbuildinfo) and
# the declarations it emits (node_modules/.tmp/tsc/) let it re-check only the
# files whose content changed and those whose dependencies' declarations did.
# That state lives in ONE checkout's node_modules, so a checkout that last
# type-checked a tree far from this one (another branch, main hours ago) starts
# nearly cold, and a fresh one starts cold: the merge gate's typecheck took a
# median 528s across the merge queue's pool checkouts (2026-10-26).
#
# So the state is also kept beside the checkouts, in a cache shared by every
# checkout of this clone on this machine, and a run starts from whichever is
# newer: its own, or the cache's. Correctness does not depend on which: tsc
# keys the state by each file's content hash and by relative paths, so state
# from another checkout of the same dependencies only changes how much it
# re-checks, never the verdict. "The same dependencies" is the cache key: the
# lockfiles, the tsconfigs and the TypeScript version.
#
# After a passing run its state becomes the cache's newest (an atomic symlink
# swap), so concurrent runs never read a half-written copy. A failing run
# saves nothing.
#
# RELIANT_TSC_CACHE=<dir> moves the cache (default: under XDG_CACHE_HOME);
# RELIANT_TSC_CACHE=off runs plain `npm run typecheck`.
set -uo pipefail
cd "$(git rev-parse --show-toplevel)/frontends/web" || exit 2

tmp=node_modules/.tmp
run() { npm run --silent typecheck; }
if [ "${RELIANT_TSC_CACHE:-}" = off ] || [ ! -d node_modules ]; then
  run
  exit
fi

common=$(git rev-parse --path-format=absolute --git-common-dir)
key=$(cat ../../package-lock.json package-lock.json tsconfig*.json node_modules/typescript/package.json 2>/dev/null | sha256sum | cut -c1-16)
cache=${RELIANT_TSC_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/reliant-web-tsc}/$(printf '%s' "$common" | sha256sum | cut -c1-12)/$key
mkdir -p "$cache"

# stamp <dir>: when that state was saved (0 when never).
stamp() { cat "$1/.saved" 2>/dev/null || echo 0; }

current=$(readlink -f "$cache/current" 2>/dev/null || true)
if [ -n "$current" ] && [ -d "$current" ] && [ "$(stamp "$current")" -gt "$(stamp "$tmp")" ]; then
  # The cache is newer: start from it. Copy beside, then swap, so an
  # interrupted copy never leaves a mixed state behind.
  rm -rf "$tmp.restore" && cp -a "$current" "$tmp.restore" 2>/dev/null &&
    rm -rf "$tmp" && mv "$tmp.restore" "$tmp" &&
    echo "web-typecheck: started from the shared incremental state ($(($(date +%s) - $(stamp "$tmp")))s old)"
  rm -rf "$tmp.restore"
fi

# A checkout whose node_modules was copied with hard links (cp -al) shares
# its incremental state, file by file, with the checkout it was copied from,
# and tsc rewrites those files in place: two gates type-checking different
# trees at once then overwrite each other's state (the merge queue's pools a
# and e did, 2026-10-26). Give this checkout its own copy first.
if [ -d "$tmp" ]; then
  find "$tmp" -type f -links +1 -print0 | while IFS= read -r -d '' f; do
    cp -p "$f" "$f.unlink" && mv -f "$f.unlink" "$f"
  done
fi

run
rc=$?
if [ "$rc" = 0 ] && [ -d "$tmp" ]; then
  date +%s >"$tmp/.saved"
  new="$cache/state.$$.$(date +%s)"
  if cp -a "$tmp" "$new" 2>/dev/null; then
    ln -sfn "$new" "$cache/current.$$" && mv -T "$cache/current.$$" "$cache/current"
  else
    rm -rf "$new"
  fi
  # Keep the current state and the one before it (a reader may be copying
  # it); older ones go.
  keep=$(readlink -f "$cache/current")
  ls -1dt "$cache"/state.* 2>/dev/null | tail -n +3 | while read -r d; do
    [ "$d" = "$keep" ] || rm -rf "$d"
  done
fi
exit "$rc"
