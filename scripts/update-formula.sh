#!/usr/bin/env bash
# Regenerate Formula/tf-slate.rb for a released tag from the release's checksums.txt.
# Usage: scripts/update-formula.sh <vX.Y.Z|X.Y.Z> [checksums-file]
# Without a checksums file, checksums.txt is downloaded (with retries) from the GitHub Release.
# Idempotent: leaves the formula untouched when it already matches.
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <version> [checksums-file]" >&2
  exit 2
fi

tag="v${1#v}"
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "version must be a plain semver like v0.2.0 (got: $tag)" >&2
  exit 2
fi
version="${tag#v}"

repo="${GITHUB_REPOSITORY:-abuxton/tf-slate}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
formula="$root/Formula/tf-slate.rb"
base="https://github.com/${repo}/releases/download/${tag}"

checksums="${2:-}"
if [[ -z "$checksums" ]]; then
  checksums="$(mktemp)"
  trap 'rm -f "$checksums"' EXIT
  for attempt in 1 2 3 4 5 6; do
    if curl -fsSL "$base/checksums.txt" -o "$checksums"; then
      break
    fi
    if [[ $attempt -eq 6 ]]; then
      echo "release checksums not available yet: $base/checksums.txt" >&2
      exit 1
    fi
    sleep $((attempt * 5))
  done
fi

# sha_for <os> <arch>: checksum of tf-slate_<version>_<os>_<arch>.tar.gz
sha_for() {
  local name="tf-slate_${version}_$1_$2.tar.gz" sha
  sha="$(awk -v n="$name" '$2 == n { print $1 }' "$checksums")"
  if [[ ! "$sha" =~ ^[0-9a-f]{64}$ ]]; then
    echo "no valid sha256 for $name in checksums" >&2
    exit 1
  fi
  echo "$sha"
}

url_for() { echo "${base}/tf-slate_${version}_$1_$2.tar.gz"; }

tmpf="$(mktemp)"
cat > "$tmpf" <<RUBY
class TfSlate < Formula
  desc "Discover, summarize, and manage local Terraform state files"
  homepage "https://github.com/${repo}"
  license "MIT"
  head "https://github.com/${repo}.git", branch: "main" do
    depends_on "go" => :build
  end

  on_macos do
    on_arm do
      url "$(url_for darwin arm64)"
      sha256 "$(sha_for darwin arm64)"
    end
    on_intel do
      url "$(url_for darwin amd64)"
      sha256 "$(sha_for darwin amd64)"
    end
  end

  on_linux do
    on_arm do
      url "$(url_for linux arm64)"
      sha256 "$(sha_for linux arm64)"
    end
    on_intel do
      url "$(url_for linux amd64)"
      sha256 "$(sha_for linux amd64)"
    end
  end

  def install
    if build.head?
      system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=HEAD"), "./cmd/tf-slate"
    else
      bin.install "tf-slate"
    end
  end

  def caveats
    <<~EOS
      Terraform is optional. It is only needed for the interactive follow-up
      commands (list, show, destroy); scanning and non-interactive output work without it.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tf-slate --version")
    assert_match "No Terraform state files found", shell_output("#{bin}/tf-slate --ni #{testpath}")
  end
end
RUBY

if cmp -s "$tmpf" "$formula"; then
  rm -f "$tmpf"
  echo "Formula already at ${tag}"
  exit 0
fi
chmod 644 "$tmpf"
mv "$tmpf" "$formula"
echo "Formula updated to ${tag}"
