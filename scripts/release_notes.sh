#!/usr/bin/env bash
# Release notes: extract the tagged version's section from CHANGELOG.md so
# tagged GitHub releases ship curated notes instead of goreleaser's raw
# commit list.
#
# Usage: release_notes.sh <output-file> [version]
#
# The version defaults to GITHUB_REF_NAME with its leading "v" stripped.
# When CHANGELOG.md has no heading for the version yet, the [Unreleased]
# section is used instead; if neither yields any content the script fails,
# so a tag can never publish empty release notes.
set -euo pipefail
cd "$(dirname "$0")/.."

out="${1:-}"
version="${2:-${GITHUB_REF_NAME:-}}"
version="${version#v}"

if [[ -z "$out" || -z "$version" ]]; then
  echo "usage: $0 <output-file> [version]  (version defaults to GITHUB_REF_NAME)" >&2
  exit 1
fi

changelog="CHANGELOG.md"
if [[ ! -f "$changelog" ]]; then
  echo "release_notes.sh: $changelog not found" >&2
  exit 1
fi

extract() {
  awk -v want="$1" '
    section && (/^## / || /^\[[^]]+\]:[[:space:]]*http/) { exit }
    !section && index($0, want) == 1 { section = 1; next }
    section { print }
  ' "$changelog"
}

notes="$(extract "## [$version]")"
if [[ -z "$(printf '%s' "$notes" | tr -d '[:space:]')" ]]; then
  notes="$(extract "## [Unreleased]")"
fi
if [[ -z "$(printf '%s' "$notes" | tr -d '[:space:]')" ]]; then
  echo "release_notes.sh: no CHANGELOG.md section for [$version] and no usable [Unreleased] fallback" >&2
  exit 1
fi

printf '%s\n' "$notes" | sed -e '/./,$!d' > "$out"
echo "release_notes.sh: wrote $out from CHANGELOG.md [$version]"
