#!/bin/sh
set -eu
script=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/prepare-env.sh
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
printf 'ADMIN_API_TOKEN=original\nMCP_UPSTREAM_TOKEN=original-upstream\nPACKRAT_DEPLOY_TOKEN=original-deploy\n' > "$tmp/dest"
cp "$tmp/dest" "$tmp/expected"
printf 'ADMIN_API_TOKEN=overwrite\n' > "$tmp/source"
sh "$script" "$tmp/source" "$tmp/dest"
cmp "$tmp/expected" "$tmp/dest"
sh "$script" "$tmp/source" "$tmp/new"
cmp "$tmp/source" "$tmp/new"
[ "$(stat -c %a "$tmp/new")" = 600 ]
if sh "$script" "$tmp/missing" "$tmp/absent" 2>/dev/null; then exit 1; fi
[ ! -e "$tmp/absent" ]
printf 'prepare-env tests passed\n'
