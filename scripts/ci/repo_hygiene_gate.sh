#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

echo "Checking repository hygiene..."

violations=()
while IFS= read -r path; do
  [ -z "$path" ] && continue
  base="$(basename "$path")"

  case "$base" in
    .DS_Store)
      violations+=("$path (macOS artifact)")
      ;;
  esac

  if [[ "$base" == ".env" || "$base" == .env.* ]] && [[ "$base" != ".env.example" ]]; then
    violations+=("$path (sensitive env file)")
  fi

  if [[ "$path" == "coverage.out" || "$path" == "coverage.html" ]]; then
    violations+=("$path (generated coverage artifact)")
  fi
done < <(git ls-files)

if [ "${#violations[@]}" -gt 0 ]; then
  echo "❌ Repository hygiene check failed. Forbidden tracked files detected:"
  for item in "${violations[@]}"; do
    echo " - $item"
  done
  echo ""
  echo "Untrack them and keep only safe templates (for example: .env.example)."
  exit 1
fi

echo "✅ Repository hygiene check passed"
