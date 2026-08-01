#!/usr/bin/env bash
set -euo pipefail

root=${1:?usage: test-reference-plugins.sh <plugins-dir>}
binary=${PLUGXFER_BINARY:-./plugxfer}

if [[ ! -x "$binary" ]]; then
  echo "plugxfer binary is not executable: $binary" >&2
  exit 1
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

count=0
for plugin in "$root"/*; do
  [[ -d "$plugin" ]] || continue
  [[ -f "$plugin/.claude-plugin/plugin.json" || -d "$plugin/skills" || -d "$plugin/commands" || -d "$plugin/agents" ]] || continue
  name=$(basename "$plugin")
  count=$((count + 1))

  set +e
  "$binary" check "$plugin" --to codex >"$work/$name-check.md" 2>"$work/$name-check.err"
  check_status=$?
  set -e
  if (( check_status > 2 )); then
    echo "check failed for $name (exit $check_status)" >&2
    cat "$work/$name-check.err" >&2
    exit 1
  fi

  set +e
  "$binary" convert "$plugin" --to codex -o "$work/$name-output" >"$work/$name-convert.md" 2>"$work/$name-convert.err"
  convert_status=$?
  set -e
  if (( convert_status > 2 )); then
    echo "convert failed for $name (exit $convert_status)" >&2
    cat "$work/$name-convert.err" >&2
    exit 1
  fi
  [[ -f "$work/$name-output/PLUGXFER-REPORT.md" ]] || {
    echo "conversion report missing for $name" >&2
    exit 1
  }

  set +e
  "$binary" convert "$work/$name-output" --to claude -o "$work/$name-roundtrip" >"$work/$name-roundtrip.md" 2>"$work/$name-roundtrip.err"
  roundtrip_status=$?
  set -e
  if (( roundtrip_status > 2 )); then
    echo "round trip failed for $name (exit $roundtrip_status)" >&2
    cat "$work/$name-roundtrip.err" >&2
    exit 1
  fi
  [[ -f "$work/$name-roundtrip/PLUGXFER-REPORT.md" ]] || {
    echo "round-trip report missing for $name" >&2
    exit 1
  }
done

if (( count == 0 )); then
  echo "no reference plugins found under $root" >&2
  exit 1
fi

echo "validated $count reference plugins"
