#!/bin/sh
# Print a draft post for X about a release, made from its CHANGELOG.md section.
# The Release workflow shows it with a link that opens X's post box; nothing is posted automatically.
# Usage: sh tools/x-post.sh v0.18.1 [CHANGELOG.md]
#
# The post is "kiroku <tag> is out:", then the first sentence of changes (Security, Added,
# Changed and Fixed first, then the rest, each in CHANGELOG order) while they fit, then the
# release URL. Removed and Security changes say so in front. X counts any URL as 23 characters
# and allows 280 in all.
set -eu
tag=${1:?usage: x-post.sh <tag> [file]}
file=${2:-CHANGELOG.md}
notes=$(sh "$(dirname "$0")/release-notes.sh" "$tag" "$file")
url="https://github.com/MichinaoShimizu/kiroku/releases/tag/$tag"
printf '%s\n' "$notes" | awk -v tag="$tag" -v url="$url" '
  function rank(s) { return s == "Security" ? 1 : s == "Added" ? 2 : s == "Changed" ? 3 : s == "Fixed" ? 4 : 5 }
  # first sentence of a bullet, without Markdown, at most 110 characters (cut at a word)
  function lead(t,   i) {
    gsub(/\*\*|`/, "", t)
    gsub(/\[[^]]*\]\([^)]*\)/, "", t)
    i = index(t, ". "); if (i) t = substr(t, 1, i - 1)
    sub(/[.:;]+$/, "", t)
    if (length(t) > 110) { t = substr(t, 1, 110); sub(/ [^ ]*$/, "", t); sub(/[ ,;:]+$/, "", t); t = t "…" }
    return t
  }
  /^### / { sec = $2; next }
  /^- / { n++; r[n] = rank(sec); x[n] = (sec == "Removed" || sec == "Security" ? sec ": " : "") lead(substr($0, 3)) }
  END {
    head = "kiroku " tag " is out:\n\n"
    used = length(head) + 2 + 23 # the blank line before the URL, and the URL as X counts it
    body = ""
    for (k = 1; k <= 5; k++) for (i = 1; i <= n; i++) if (r[i] == k && x[i] != "") {
      line = "- " x[i] "\n"
      if (used + length(line) > 280) continue
      body = body line; used += length(line)
    }
    if (body == "") head = "kiroku " tag " is out.\n"
    printf "%s%s\n%s\n", head, body, url
  }'
