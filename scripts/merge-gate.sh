#!/usr/bin/env bash
# merge-gate.sh — reliant's MERGE tier: only what this change can affect, in
# about five minutes. `task ci:merge` runs it on your branch; the merge queue
# runs it on the squash it is about to land. What it leaves out runs in the
# release gate (scripts/release-gate.sh), on main, before a release is cut.
# dev-docs/merge-and-release-gates.md is the design and the trade-offs.
#
#   scripts/merge-gate.sh [--base <rev>] [--amend] [--regen]
#
#   --base <rev>  diff base (default: the merge-base of HEAD and origin/main)
#   --amend       a regenerate that changes only generated paths is amended
#                 into HEAD instead of failing (the merge queue's mode: the
#                 merge with main can leave generated files a step behind)
#   --regen       run forge generate even if no forge input changed (the
#                 merge queue resolved a generated-file conflict)
#
# Phases, each a row of the summary (slot wait vs run, seconds):
#   plan       tools/test-affected plan: the ONE rule set (dev-docs/testing-affected.md)
#   npm-*      npm ci where a selected lane needs node_modules it lacks
#   generate   pinned forge generate must be a no-op; only when a forge input changed
#   docs       every docs generator must be a no-op; only when a docs input changed
#   build      go build ./...
#   test       tools/test-affected run -lanes go,web,electron (-short, cached, no -race)
#   lint       forge lint --scope <changed dirs> (unscoped when a pin moved)
#   migsafety  forge lint --migration-safety, only when a migration changed
# generate and docs run first, because they can change the tree the others
# judge; build, test, lint and migsafety then run at the same time.
#
# The whole gate holds ONE host heavy slot (forge slot run), taken here; every
# phase nests inside it and never queues again. MG_LOGDIR=<dir> keeps the
# phase logs there (default: a fresh directory under ~/.cache/reliant-merge-gate).
set -uo pipefail
cd "$(git rev-parse --show-toplevel)" || exit 2

base="" amend=0 regen=0
while [ $# -gt 0 ]; do
  case "$1" in
    --base) base=$2; shift 2 ;;
    --amend) amend=1; shift ;;
    --regen) regen=1; shift ;;
    -h|--help) sed -n '2,32p' "$0"; exit 0 ;;
    *) echo "merge-gate: unknown argument $1" >&2; exit 2 ;;
  esac
done

export GOFLAGS="${GOFLAGS:+$GOFLAGS }-trimpath" GOPRIVATE="${GOPRIVATE:-github.com/reliant-labs/*}"
# A gate must not rewrite the clone's git config (forge's merge driver).
export FORGE_NO_MERGE_DRIVER=1

# ── one heavy slot for the whole gate ─────────────────────────────────────
if [ -z "${FORGE_SLOT_HELD:-}" ] && [ -z "${MG_IN_SLOT:-}" ] && forge slot status --class heavy >/dev/null 2>&1; then
  MG_SLOT_T0=$(date +%s) MG_IN_SLOT=1 exec forge slot run --class heavy -- "$0" ${base:+--base "$base"} $([ "$amend" = 1 ] && echo --amend) $([ "$regen" = 1 ] && echo --regen)
fi
t0=$(date +%s)
slot_wait=$(( t0 - ${MG_SLOT_T0:-$t0} ))

[ -n "$base" ] || base=$(git merge-base HEAD origin/main) || { echo "merge-gate: no merge-base with origin/main (fetch it, or pass --base)" >&2; exit 2; }
base=$(git rev-parse "$base")
L=${MG_LOGDIR:-${XDG_CACHE_HOME:-$HOME/.cache}/reliant-merge-gate/$(date -u +%Y%m%dT%H%M%SZ)-$$}
mkdir -p "$L"
echo "$slot_wait" >"$L/slot_wait"
pin=$(sed -n 's/^forge_version:[[:space:]]*//p' forge.yaml | head -1)
if [ -n "${FORGE_BIN:-}" ]; then forge=("$FORGE_BIN"); else forge=(env GOFLAGS=-trimpath go run "github.com/reliant-labs/forge/cmd/forge@$pin"); fi
# Paths generators write. A regenerate in --amend mode may change only these.
gen_re='^gen/|^generated/|^docs/reference/[^/]*\.mdx$|^docs/changelog\.mdx$|^frontends/web/src/gen/|^\.forge/|^deploy/observability/|_gen(_test)?\.(go|json|ndjson|ts|k)$|^frontends/web/src/store/shortcutsData\.generated\.ts$|^internal/workflow/runtime/schema/field_descriptions\.go$|^internal/skills/catalog/builtin/workflow-builder/SKILL\.md$'

rows=() notes=() fail=0
# phase <name> <cmd>...: run one phase, timed, output to $L/<name>.log.
phase() {
  local n=$1 s e rc; shift
  s=$(date +%s)
  "$@" >"$L/$n.log" 2>&1; rc=$?
  e=$(date +%s)
  echo "$n $rc $((e - s)) $((s - t0))" >"$L/$n.row"
  return $rc
}
snapshot() {
  git ls-files -z --modified --others --exclude-standard | sort -zu |
    while IFS= read -r -d '' f; do
      if [ -e "$f" ] || [ -L "$f" ]; then printf '%s\t%s\n' "$f" "$(git hash-object --no-filters -- "$f")"; else printf '%s\tdeleted\n' "$f"; fi
    done
}
# judge <phase> <snapshot before>: did the generator change the tree?
judge() {
  local n=$1 before=$2 changed bad
  changed=$(diff <(printf '%s\n' "$before") <(snapshot) | awk -F'\t' '/^[<>] / { print substr($1, 3) }' | sort -u | grep -v '^$' || true)
  [ -z "$changed" ] && return 0
  bad=$(echo "$changed" | grep -Ev "$gen_re" || true)
  if [ "$amend" = 1 ] && [ -z "$bad" ]; then
    git add -A && git -c user.name="${GIT_AUTHOR_NAME:-reliant-merge-gate}" -c user.email="${GIT_AUTHOR_EMAIL:-merge-gate@reliantlabs.io}" commit -q --amend --no-edit --no-verify
    notes+=("$n: amended $(echo "$changed" | wc -l | tr -d ' ') regenerated file(s) into HEAD")
    return 0
  fi
  {
    echo "$n is not a no-op: it changed $(echo "$changed" | wc -l | tr -d ' ') file(s):"
    echo "$changed" | sed 's/^/  /'
    [ -n "$bad" ] && [ "$amend" = 1 ] && { echo "of which NOT generated paths (the branch must commit them):"; echo "$bad" | sed 's/^/  /'; }
    echo "Commit what it wrote (it is in your tree), in the change that caused it."
  } >>"$L/$n.log"
  [ "$amend" = 1 ] && { git reset -q --hard; git clean -fdq; }
  return 1
}

# ── plan ──────────────────────────────────────────────────────────────────
phase plan go run ./tools/test-affected plan -json -base "$base" || { cat "$L/plan.log"; exit 1; }
sed -n '/^{/,$p' "$L/plan.log" >"$L/plan.json"
j() { jq -r "$1" "$L/plan.json"; }
gofull=$(j .go.full) ngo=$(j '.go.packages|length') webfull=$(j .web.full) nweb=$(j '.web.tests|length')
elec=$(j .electron.full) fgen=$(j .forge_generate.full) dgen=$(j .docs_generate.full)
[ "$regen" = 1 ] && fgen=true
notes+=("plan: go $([ "$gofull" = true ] && echo FULL || echo "$ngo package(s)"); web $([ "$webfull" = true ] && echo FULL || echo "$nweb file(s)"); electron $elec; forge-generate $fgen; docs-generate $dgen")

# ── npm installs a selected lane needs ────────────────────────────────────
need=()
if [ "$webfull" = true ] || [ "$nweb" -gt 0 ]; then need+=(. frontends/web); fi
if [ "$elec" = true ]; then need+=(. electron); fi
if [ "$fgen" = true ]; then need+=(tools/protoc-gen-es); fi
for d in $(printf '%s\n' "${need[@]}" | sort -u); do
  [ -f "$d/package-lock.json" ] || continue
  [ "$d/node_modules/.package-lock.json" -nt "$d/package-lock.json" ] && continue
  n=npm-$(echo "$d" | tr '/.' '-r')
  phase "$n" npm ci --prefix "$d" --ignore-scripts --no-audit --no-fund --loglevel=error || fail=1
done

# ── generate, then docs: they may change what everything after judges ─────
if [ "$fail" = 0 ] && [ "$fgen" = true ]; then
  b=$(snapshot)
  # task generate:forge, exactly as `task generate:check` runs it: the pinned
  # forge (FORGE_BIN when set), the pinned protoc-gen-es, go.mod's toolchain.
  phase generate task generate:forge && judge generate "$b" || fail=1
  [ "$fail" = 0 ] && { phase conventions "${forge[@]}" lint --no-fix --conventions || fail=1; }
fi
if [ "$fail" = 0 ] && [ "$dgen" = true ]; then
  b=$(snapshot)
  phase docs task generate:docs && judge docs "$b" || fail=1
fi

# ── build ‖ test ‖ lint ‖ migsafety ────────────────────────────────────────
if [ "$fail" = 0 ]; then
  files=$( { git diff --name-only "$base"; git ls-files --others --exclude-standard; } | sort -u)
  pids=()
  phase build go build ./... & pids+=($!)
  phase test go run ./tools/test-affected run -base "$base" -lanes go,web,electron & pids+=($!)
  if echo "$files" | grep -qE '^(go\.(mod|sum)|forge\.yaml)$'; then
    notes+=("lint: unscoped (a pin moved)")
    phase lint "${forge[@]}" lint --quiet --no-fix --skip-frontends & pids+=($!)
  else
    scope=$(echo "$files" | grep / | xargs -r -n1 dirname | sort -u | while read -r d; do [ -d "$d" ] && echo "$d"; done | paste -sd, -)
    if [ -n "$scope" ]; then
      notes+=("lint: --scope over $(echo "$scope" | tr ',' '\n' | wc -l | tr -d ' ') changed dir(s)")
      phase lint "${forge[@]}" lint --quiet --no-fix --skip-frontends --scope "$scope" & pids+=($!)
    else
      notes+=("lint: not run (only root files changed)")
    fi
  fi
  if echo "$files" | grep -qE '^db/migrations/|^forge\.yaml$'; then
    phase migsafety "${forge[@]}" lint --migration-safety --no-fix & pids+=($!)
  fi
  for p in "${pids[@]}"; do wait "$p" || fail=1; done
fi

# ── summary ───────────────────────────────────────────────────────────────
end=$(date +%s)
{
  echo "### merge-gate: $([ "$fail" = 0 ] && echo PASSED || echo FAILED)"
  echo
  echo "| phase | result | seconds | started +s |"
  echo "|---|---|---|---|"
  echo "| heavy slot (queue) | - | $slot_wait | - |"
  for f in plan npm-r npm-frontends-web npm-electron npm-tools-protoc-gen-es generate conventions docs build test lint migsafety; do
    [ -f "$L/$f.row" ] || continue
    read -r n rc s st <"$L/$f.row"
    echo "| $n | $([ "$rc" = 0 ] && echo PASS || echo "FAIL ($rc)") | $s | $st |"
  done
  echo
  echo "wall: $((end - t0))s in the slot, after ${slot_wait}s queued for it; diff base $(git rev-parse --short "$base")"
  for n in "${notes[@]}"; do echo "- $n"; done
  echo "head: $(git rev-parse --short HEAD)  logs: $L"
} >"$L/summary.md"
if [ "$fail" != 0 ]; then
  for f in "$L"/*.row; do
    read -r n rc _ _ <"$f"
    [ "$rc" = 0 ] && continue
    echo "── $n failed (rc $rc), last lines of $L/$n.log:"
    grep -v '^ok \|no test files\|^[[:space:]]*$' "$L/$n.log" | tail -25 | sed 's/^/   /'
  done
fi
cat "$L/summary.md"
exit "$fail"
