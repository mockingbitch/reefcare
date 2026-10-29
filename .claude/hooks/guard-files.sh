#!/usr/bin/env bash
# PreToolUse: block edits that violate project rules.
#  - sqlc-generated Go files (regenerate with `sqlc generate` instead)
#  - migrations already committed to git (add a new migration instead)
#  - environment/secret files
# Exit code 2 blocks the tool call and shows stderr to Claude.
set -euo pipefail

command -v jq >/dev/null 2>&1 || exit 0

file=$(jq -r '.tool_input.file_path // empty')
[[ -n "$file" ]] || exit 0

root="${CLAUDE_PROJECT_DIR:-$(pwd)}"
rel="${file#"$root"/}"

case "$rel" in
  .env.example|*/.env.example) ;;
  .env|.env.*|*/.env|*/.env.*)
    echo "Blocked: do not edit environment/secret files ($rel). Update .env.example instead." >&2
    exit 2
    ;;
esac

if [[ "$rel" == *.go && -f "$file" ]] && head -n 5 "$file" | grep -q "Code generated .* DO NOT EDIT"; then
  echo "Blocked: $rel is generated code. Change db/queries or db/migrations and run 'sqlc generate'." >&2
  exit 2
fi

if [[ "$rel" == db/migrations/* && -f "$file" ]]; then
  if git -C "$root" ls-files --error-unmatch "$rel" >/dev/null 2>&1; then
    echo "Blocked: $rel is already committed and may have been applied. Create a new Goose migration instead." >&2
    exit 2
  fi
fi

exit 0
