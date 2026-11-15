#!/bin/sh
# Existing deployed configuration is authoritative: never replace its secrets
# from a checkout. First-install configuration is copied atomically and private.
set -eu
src=${1:?source env path required}
dst=${2:?deployed env path required}
if [ -e "$dst" ]; then
    [ -f "$dst" ] && [ -r "$dst" ] || { echo 'Deployed env is not a readable regular file' >&2; exit 1; }
    exit 0
fi
[ -f "$src" ] && [ -r "$src" ] || { echo 'First deployment requires a readable source env file' >&2; exit 1; }
umask 077
tmp=$(mktemp "${dst}.tmp.XXXXXX")
trap 'rm -f "$tmp"' EXIT HUP INT TERM
cat "$src" > "$tmp"
# Link rather than overwrite if another process initialized it concurrently.
if ! ln "$tmp" "$dst" 2>/dev/null; then
    [ -f "$dst" ] && [ -r "$dst" ] || { echo 'Unable to initialize deployed env' >&2; exit 1; }
fi
