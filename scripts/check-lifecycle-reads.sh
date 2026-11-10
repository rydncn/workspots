#!/bin/sh
# Rule 19: lifecycle fields are read only where their condition is defined or
# moved. Finds every comparison of status, hidden or expires_at in Go code and
# templates, removes the reads scripts/lifecycle-reads.txt allows, and fails on
# what is left; also fails on an allowed read that matches nothing, so a moved
# read cannot hide behind a stale entry. Run from the repository root.
set -eu
pattern="status[[:space:]]*(=|<>)[[:space:]]*('|\\\$)|NOT[[:space:]]+(r\\.)?hidden|hidden[[:space:]]*=[[:space:]]*(true|false)|expires_at[[:space:]]*(<=|<|>=|>|=)|(eq|ne) [^}]*Status"
hits=$(grep -rnE "$pattern" internal cmd --include='*.go' --include='*.html' --exclude='*_test.go' || true)
test -n "$hits" || { echo "no lifecycle read found at all: the pattern or the folders are wrong"; exit 1; }
status=0
while IFS= read -r line; do
  case $line in ''|'#'*) continue;; esac
  fragment=${line%%	*}
  if ! printf '%s\n' "$hits" | grep -qF -- "$fragment"; then
    echo "allowed read no longer exists: $fragment"; status=1
  fi
  hits=$(printf '%s\n' "$hits" | grep -vF -- "$fragment" || true)
done < scripts/lifecycle-reads.txt
if [ -n "$hits" ]; then
  echo "lifecycle field read outside its definition (docs/design.md, Lifecycles; list a deliberate one in scripts/lifecycle-reads.txt):"
  echo "$hits"; status=1
fi
exit $status
