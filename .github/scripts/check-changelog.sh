#!/usr/bin/env bash
# Fail when a PR changes production code under cmd/ or internal/ without
# touching CHANGELOG.md. The `skip-changelog` label bypasses the check.
set -euo pipefail

: "${BASE_SHA:?}" "${HEAD_SHA:?}"
labels="${PR_LABELS:-}"

if [[ ",${labels}," == *",skip-changelog,"* ]]; then
  echo "skip-changelog label set; skipping."
  exit 0
fi

changed="$(git diff --name-only "${BASE_SHA}...${HEAD_SHA}")"

code="$(grep -E '^(cmd|internal)/.*\.go$' <<<"${changed}" | grep -vE '_test\.go$' || true)"
if [[ -z "${code}" ]]; then
  echo "No production code changed; changelog entry not required."
  exit 0
fi

if grep -qx 'CHANGELOG.md' <<<"${changed}"; then
  echo "CHANGELOG.md updated."
  exit 0
fi

echo "::error file=CHANGELOG.md::Production code changed but CHANGELOG.md was not updated. Add an entry under [Unreleased], or apply the 'skip-changelog' label if the change is not user-visible."
echo "Changed production files:"
echo "${code}"
exit 1
