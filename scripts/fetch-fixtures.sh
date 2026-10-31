#!/usr/bin/env bash
# Usage: scripts/fetch-fixtures.sh [sources.tsv] [outdir]
# Downloads each "slug<TAB>url" line as raw HTML. Skips files that already exist.
set -u
src="${1:-testdata/sources.tsv}"
out="${2:-testdata/fixtures}"
ua='Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0'
mkdir -p "$out"
fetch() {
  slug="$1"; url="$2"; dest="$out/$slug.html"
  [ -s "$dest" ] && { echo "skip  $slug"; return; }
  code=$(curl -sL -m 40 --compressed -A "$ua" -H 'Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8' \
    -H 'Accept-Language: en-US,en;q=0.5' -H 'Upgrade-Insecure-Requests: 1' \
    -H 'Sec-Fetch-Dest: document' -H 'Sec-Fetch-Mode: navigate' -H 'Sec-Fetch-Site: none' \
    -w '%{http_code}' -o "$dest.tmp" "$url")
  size=$(stat -c %s "$dest.tmp" 2>/dev/null || echo 0)
  if [ "$code" = 200 ] && [ "$size" -gt 20000 ]; then
    mv "$dest.tmp" "$dest"; echo "ok    $slug ($size)"
  else
    rm -f "$dest.tmp"; echo "FAIL  $slug http=$code size=$size"
  fi
}
export -f fetch; export out ua
while IFS=$'\t' read -r slug url; do
  [ -z "$slug" ] && continue
  printf '%s\0%s\0' "$slug" "$url"
done < "$src" | xargs -0 -n2 -P8 bash -c 'fetch "$0" "$1"'
