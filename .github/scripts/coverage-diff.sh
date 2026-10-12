#!/usr/bin/env bash
# Informational coverage comparison between the PR base and the checked-out tree.
# Never fails the build: problems are reported in the summary and the exit code is 0.
#
# Required environment: BASE_SHA. Optional: GITHUB_STEP_SUMMARY (else stdout).
set -uo pipefail

: "${BASE_SHA:?}"
out="${GITHUB_STEP_SUMMARY:-/dev/stdout}"
work="$(mktemp -d)"
trap 'git worktree remove --force "$work/base" >/dev/null 2>&1 || true; rm -rf "$work"' EXIT

# coverage <dir> <label>: prints "<package> <percent>" lines, then "TOTAL <percent>".
# Prints nothing and returns 1 when tests fail to run.
coverage() {
  local dir="$1" label="$2" profile="$work/$2.out" log="$work/$2.log"
  if ! (cd "$dir" && go test -covermode=atomic -coverprofile="$profile" ./... >"$log" 2>&1); then
    return 1
  fi
  awk '/coverage:/ { for (i = 1; i <= NF; i++) if ($i == "coverage:") { sub(/%/, "", $(i+1)); print $2, $(i+1) } }' "$log"
  (cd "$dir" && go tool cover -func="$profile" | awk '/^total:/ { sub(/%/, "", $NF); print "TOTAL", $NF }')
}

{
  echo "## Coverage (informational)"
  echo
} >>"$out"

if ! git worktree add --detach "$work/base" "$BASE_SHA" >/dev/null 2>&1; then
  echo "Base commit \`${BASE_SHA:0:7}\` could not be checked out; no comparison available." >>"$out"
  exit 0
fi

head_cov="$(coverage . head)" || {
  echo "Tests did not pass on this PR, so coverage is unavailable. See the CI workflow." >>"$out"
  exit 0
}
base_cov="$(coverage "$work/base" base)" || base_cov=""

if [ -z "$base_cov" ]; then
  echo "Base \`${BASE_SHA:0:7}\` has no usable test run; showing this PR only." >>"$out"
  echo >>"$out"
fi

awk -v base="$base_cov" '
  function fmt(v) { return (v == "") ? "n/a" : sprintf("%.1f%%", v) }
  BEGIN {
    n = split(base, lines, "\n")
    for (i = 1; i <= n; i++) { split(lines[i], f, " "); if (f[1] != "") b[f[1]] = f[2] }
    print "| Package | Base | This PR | Change |"
    print "| --- | ---: | ---: | ---: |"
  }
  {
    name = $1; h = $2; seen[name] = 1
    if (name == "TOTAL") { total = h; next }
    sub(/^github\.com\/[^\/]+\/[^\/]+\//, "", name)
    d = (b[$1] == "") ? "new" : sprintf("%+.1f", h - b[$1])
    print "| `" name "` | " fmt(b[$1]) " | " fmt(h) " | " d " |"
  }
  END {
    for (k in b) if (!(k in seen) && k != "TOTAL") {
      name = k; sub(/^github\.com\/[^\/]+\/[^\/]+\//, "", name)
      print "| `" name "` | " fmt(b[k]) " | removed | |"
    }
    d = (b["TOTAL"] == "") ? "" : sprintf("%+.1f", total - b["TOTAL"])
    print "| **Total** | " fmt(b["TOTAL"]) " | **" fmt(total) "** | " d " |"
  }
' <<<"$head_cov" >>"$out"

echo >>"$out"
echo "_Informational only: this check never fails a PR._" >>"$out"
exit 0
