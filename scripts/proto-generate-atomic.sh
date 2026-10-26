#!/usr/bin/env bash
# proto-generate-atomic.sh — run `buf generate` so a failure can never leave the
# tree without generated code.
#
# Usage: scripts/proto-generate-atomic.sh <buf-template> <out-dir> [<out-dir>...]
# `forge generate` runs it as the `controlplane-client` hook (forge.yaml
# generate.hooks), with the pinned plugins and Go toolchain.
#
# The out-dirs are the template's plugin `out:` directories, relative to the
# repo root. They are regenerated WHOLESALE: anything not produced by this run
# is gone afterwards, which is how stale files from removed protos get pruned.
#
# Why not `rm -rf <out-dirs> && buf generate`: that order deletes first and
# generates second, so any generate failure (buf.build rate limiting, a
# network blip, a bad proto) left gen/controlplane and frontends/web/src/gen/controlplane
# deleted — a tree that no longer builds, with nothing to roll back to.
#
# Instead buf writes into a scratch directory (`buf generate -o`, which
# prefixes every plugin's out: path), and only after it succeeds — and every
# expected out-dir actually came out non-empty — is each live directory
# replaced by its freshly generated twin. A failure anywhere before the swap
# exits non-zero with the existing directories untouched.
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <buf-template> <out-dir> [<out-dir>...]" >&2
  exit 2
fi

template=$1
shift

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

for dir in "$@"; do
  case $dir in
    /* | *..* | "" | .)
      echo "proto-generate-atomic: out-dir must be a relative path inside the repo, got '$dir'" >&2
      exit 2
      ;;
  esac
done

# ── Skip a regeneration that cannot change anything ─────────────────────
# buf over the whole vendored contract costs ~50s on a loaded host, and every
# `forge generate` runs this hook, while its inputs move only when the contract
# is re-vendored. So a run whose inputs AND outputs are byte-identical to those
# of an earlier successful run on this machine skips: the generator is a pure
# function of the inputs fingerprinted below, so it would write the outputs it
# wrote then, which are the outputs on disk now. A hand edit of an output
# changes the output fingerprint and regenerates. The record is shared by every
# checkout (keyed by content, not path). PROTO_GENERATE_FORCE=1 always runs.
#
# Inputs: this script, the template, every file under the template's input
# directories, the Go and npm pins of buf and the plugins, BUF, and the Go
# toolchain.
fingerprint_inputs() {
  local d
  {
    sha256sum "${BASH_SOURCE[0]}" "$template" go.mod go.sum tools/protoc-gen-es/package-lock.json
    printf 'BUF=%s\n' "${BUF:-}"
    go env GOVERSION
    for d in $(sed -n 's/^[[:space:]]*-[[:space:]]*directory:[[:space:]]*//p' "$template"); do
      find "$d" -type f -print0 | sort -z | xargs -0 -r sha256sum
    done
    true
  } 2>/dev/null | sha256sum | cut -c1-32
}
fingerprint_outputs() {
  local d
  for d in "$@"; do
    if [[ -d "$d" ]]; then find "$d" -type f -print0 | sort -z | xargs -0 -r sha256sum; fi
  done | sha256sum | cut -c1-32
}
record_dir=${XDG_CACHE_HOME:-$HOME/.cache}/reliant-proto-generate
in_fp=$(fingerprint_inputs)
record="$record_dir/$(basename "$template").$in_fp"
if [[ "${PROTO_GENERATE_FORCE:-}" != 1 ]] && [[ -f "$record" ]] && [[ "$(cat "$record")" == "$(fingerprint_outputs "$@")" ]]; then
  echo "proto-generate-atomic: $template's inputs and outputs are unchanged since a verified run; skipped (PROTO_GENERATE_FORCE=1 regenerates)"
  exit 0
fi

# The scratch dir lives inside the repo (gitignored tmp/) so the final mv is a
# same-filesystem rename, not a cross-device copy.
mkdir -p tmp
staging=$(mktemp -d "$repo_root/tmp/proto-generate.XXXXXX")
trap 'rm -rf "$staging"' EXIT

# buf is go.mod's `tool github.com/bufbuild/buf/cmd/buf` pin, the one forge
# runs too. `go run pkg@version` builds it in its own module graph (reliant's
# newer protovalidate does not build buf v1.57.0, so `go tool buf` cannot);
# GOFLAGS is reset to -trimpath because `go run pkg@version` refuses a
# repo-level -mod. BUF=<command> overrides it. The templates' plugins are all
# local and resolve relative to the repo root.
if [[ -n "${BUF:-}" ]]; then
  read -r -a buf <<<"$BUF"
else
  buf_version=$(GOWORK=off go list -m -f '{{.Version}}' github.com/bufbuild/buf)
  buf=(env GOWORK=off GOFLAGS=-trimpath go run "github.com/bufbuild/buf/cmd/buf@${buf_version}")
fi
"${buf[@]}" generate --template "$template" -o "$staging"

for dir in "$@"; do
  if [[ ! -d "$staging/$dir" ]] || [[ -z "$(ls -A "$staging/$dir")" ]]; then
    echo "proto-generate-atomic: buf generate succeeded but produced nothing in '$dir'; leaving the existing tree untouched" >&2
    exit 1
  fi
done

# Every out-dir generated: swap them in. Each live dir is moved aside before
# its replacement moves in, so no instant has it missing-and-unrecoverable.
for dir in "$@"; do
  mkdir -p "$(dirname "$dir")"
  if [[ -e "$dir" ]]; then
    mv "$dir" "$staging/.previous.$(echo "$dir" | tr '/' '_')"
  fi
  mv "$staging/$dir" "$dir"
done

# Record what these inputs generate, for the skip above. Best effort: a
# record that cannot be written only costs the next run its skip.
mkdir -p "$record_dir" 2>/dev/null &&
  fingerprint_outputs "$@" >"$record.$$" 2>/dev/null &&
  mv -f "$record.$$" "$record" 2>/dev/null || rm -f "$record.$$" 2>/dev/null || true
