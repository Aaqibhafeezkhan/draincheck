#!/usr/bin/env bash
# Print the CHANGELOG.md section for a release tag, for `goreleaser --release-notes`.
# Usage: release-notes.sh vX.Y.Z [CHANGELOG.md]
# Fails when the tag is not vX.Y.Z or when its section is missing or empty, so a
# release never ships with generated or blank notes.
set -euo pipefail

tag="${1:-}"
file="${2:-CHANGELOG.md}"

if [[ ! "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "release-notes.sh: tag must look like vX.Y.Z, got '${tag}'" >&2
  exit 2
fi
version="${tag#v}"

# From the "## [X.Y.Z]" heading to the next "## [" heading or the link
# references at the end of the file; heading excluded, blank edges trimmed.
notes="$(awk -v heading="## [${version}]" '
  index($0, heading) == 1 { in_section = 1; next }
  in_section && (/^## \[/ || /^\[[^]]+\]: /) { exit }
  in_section { print }
' "${file}" | awk 'NF { started = 1 } started { lines[++n] = $0 }
  END { while (n > 0 && lines[n] == "") n--; for (i = 1; i <= n; i++) print lines[i] }')"

if [[ -z "${notes}" ]]; then
  echo "release-notes.sh: no non-empty section '## [${version}]' in ${file}" >&2
  exit 1
fi

printf '%s\n' "${notes}"
