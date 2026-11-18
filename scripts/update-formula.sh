#!/usr/bin/env bash
# Update Formula/tf-slate.rb to a released tag.
# Usage: scripts/update-formula.sh <vX.Y.Z|X.Y.Z> [sha256]
# Without a checksum, the tag archive is downloaded (with retries) and hashed.
# Idempotent: exits 0 without changes when the formula already matches.
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <version> [sha256]" >&2
  exit 2
fi

tag="v${1#v}"
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "version must be a plain semver like v0.2.0 (got: $tag)" >&2
  exit 2
fi

repo="${GITHUB_REPOSITORY:-abuxton/tf-slate}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
formula="$root/Formula/tf-slate.rb"
url="https://github.com/${repo}/archive/refs/tags/${tag}.tar.gz"

sha="${2:-}"
if [[ -z "$sha" ]]; then
  tmp="$(mktemp)"
  trap 'rm -f "$tmp"' EXIT
  for attempt in 1 2 3 4 5 6; do
    if curl -fsSL "$url" -o "$tmp"; then
      break
    fi
    if [[ $attempt -eq 6 ]]; then
      echo "release archive not available yet: $url" >&2
      exit 1
    fi
    sleep $((attempt * 5))
  done
  sha="$(shasum -a 256 "$tmp" | awk '{print $1}')"
fi

if [[ ! "$sha" =~ ^[0-9a-f]{64}$ ]]; then
  echo "invalid sha256: $sha" >&2
  exit 2
fi

if grep -q "url \"$url\"" "$formula" && grep -q "sha256 \"$sha\"" "$formula"; then
  echo "Formula already at ${tag}"
  exit 0
fi

tmpf="$(mktemp)"
awk -v url="$url" -v sha="$sha" '
  /^  url "/ { print "  url \"" url "\""; next }
  /^  sha256 "/ { print "  sha256 \"" sha "\""; next }
  { print }
' "$formula" > "$tmpf"
mv "$tmpf" "$formula"
echo "Formula updated to ${tag} (${sha})"
