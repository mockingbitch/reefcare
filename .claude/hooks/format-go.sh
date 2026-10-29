#!/usr/bin/env bash
# PostToolUse: format Go files after Claude edits them.
# Silently no-ops when jq/gofmt are unavailable so it never blocks work.
set -euo pipefail

command -v jq >/dev/null 2>&1 || exit 0

file=$(jq -r '.tool_input.file_path // empty')
[[ -n "$file" && "$file" == *.go && -f "$file" ]] || exit 0

# Generated sqlc code must not be touched.
if head -n 5 "$file" | grep -q "Code generated .* DO NOT EDIT"; then
  exit 0
fi

if command -v goimports >/dev/null 2>&1; then
  goimports -w "$file"
elif command -v gofmt >/dev/null 2>&1; then
  gofmt -w "$file"
fi
exit 0
