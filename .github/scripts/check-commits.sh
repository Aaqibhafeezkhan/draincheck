#!/usr/bin/env bash
# Checks every commit of a pull request:
#   1. it has a "Signed-off-by:" trailer whose e-mail matches the commit author
#      (what `git commit -s` writes);
#   2. GitHub reports its signature as verified (what `git commit -S` produces,
#      once the key is registered on the account).
# Merge commits and bot accounts are skipped.
#
# Required environment: BASE_SHA HEAD_SHA PR_NUMBER REPO (and GH_TOKEN for gh).
set -euo pipefail

: "${BASE_SHA:?}" "${HEAD_SHA:?}" "${PR_NUMBER:?}" "${REPO:?}"

BOT_SUFFIX='[bot]@users.noreply.github.com'
failed=0

fail() {
  echo "::error::$1"
  failed=1
}

# --- 1. Signed-off-by -------------------------------------------------------
for sha in $(git rev-list --reverse "${BASE_SHA}..${HEAD_SHA}"); do
  short="${sha:0:7}"
  author_email="$(git show -s --format=%ae "$sha")"
  subject="$(git show -s --format=%s "$sha")"
  parents="$(git show -s --format=%P "$sha")"

  if [[ "$parents" == *" "* ]]; then
    echo "skip   $short merge commit"
    continue
  fi
  if [[ "$author_email" == *"$BOT_SUFFIX" ]]; then
    echo "skip   $short bot commit"
    continue
  fi

  signoffs="$(git show -s --format=%B "$sha" | git interpret-trailers --parse |
    sed -n 's/^[Ss]igned-off-by:.*<\(.*\)>[[:space:]]*$/\1/p' | tr 'A-Z' 'a-z')"
  want="$(printf '%s' "$author_email" | tr 'A-Z' 'a-z')"

  if grep -qxF -- "$want" <<<"$signoffs"; then
    echo "ok     $short signed off: $subject"
  else
    fail "$short has no 'Signed-off-by: ... <$author_email>' trailer (use: git commit -s). Subject: $subject"
  fi
done

# --- 2. Verified signature --------------------------------------------------
rows="$(gh api --paginate "repos/${REPO}/pulls/${PR_NUMBER}/commits" \
  --jq '.[] | [.sha, (.commit.verification.verified | tostring), .commit.verification.reason, (.commit.author.email // ""), (.commit.message | split("\n")[0])] | @tsv')"

while IFS=$'\t' read -r sha verified reason author_email subject; do
  [ -n "$sha" ] || continue
  short="${sha:0:7}"
  if [[ "$author_email" == *"$BOT_SUFFIX" ]]; then
    continue
  fi
  if [ "$verified" = "true" ]; then
    echo "ok     $short signature verified"
  else
    fail "$short signature is not verified (reason: $reason). Sign with 'git commit -S' and add your key to GitHub. Subject: $subject"
  fi
done <<<"$rows"

if [ "$failed" -ne 0 ]; then
  echo "Commit checks failed."
  exit 1
fi
echo "All commits are signed off and verified."
