#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ -z "$version" ]]; then
  echo "usage: scripts/verify-release.sh X.Y.Z" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

changelog="CHANGELOG.md"
if ! rg -q "^## ${version} - " "$changelog"; then
  echo "missing changelog section for $version" >&2
  exit 2
fi
if rg -q "^## ${version} - Unreleased" "$changelog"; then
  echo "changelog section still Unreleased for $version" >&2
  exit 2
fi

notes_file="$(mktemp -t gogcli-release-notes)"
awk -v ver="$version" '
  $0 ~ "^## "ver" " {print "## "ver; in_section=1; next}
  in_section && /^## / {exit}
  in_section {print}
' "$changelog" | sed '/^$/d' > "$notes_file"

if [[ ! -s "$notes_file" ]]; then
  echo "release notes empty for $version" >&2
  exit 2
fi

release_body="$(gh release view "v$version" --json body -q .body)"
if [[ -z "$release_body" ]]; then
  echo "GitHub release notes empty for v$version" >&2
  exit 2
fi

assets_count="$(gh release view "v$version" --json assets -q '.assets | length')"
if [[ "$assets_count" -eq 0 ]]; then
  echo "no GitHub release assets for v$version" >&2
  exit 2
fi

release_run_id="$(gh run list -L 20 --workflow release.yml --json databaseId,conclusion,headBranch -q ".[] | select(.headBranch==\"v$version\") | select(.conclusion==\"success\") | .databaseId" | head -n1)"
if [[ -z "$release_run_id" ]]; then
  echo "release workflow not green for v$version" >&2
  exit 2
fi

ci_ok="$(gh run list -L 1 --workflow ci --branch main --json conclusion -q '.[0].conclusion')"
if [[ "$ci_ok" != "success" ]]; then
  echo "CI not green for main" >&2
  exit 2
fi

make ci

sha_url="https://github.com/steipete/gogcli/archive/refs/tags/v${version}.tar.gz"
sha_file="/tmp/gogcli-${version}.tar.gz"
rm -f "$sha_file"
curl -L -o "$sha_file" "$sha_url"
sha256="$(shasum -a 256 "$sha_file" | awk '{print $1}')"

formula_path="../homebrew-tap/Formula/gogcli.rb"
if [[ ! -f "$formula_path" ]]; then
  echo "missing formula at $formula_path" >&2
  exit 2
fi

formula_url="$(rg -m1 '^\s*url ' "$formula_path" | sed -E 's/\s*url "([^"]+)"/\1/' | xargs)"
formula_sha="$(rg -m1 '^\s*sha256 ' "$formula_path" | sed -E 's/\s*sha256 "([^"]+)"/\1/' | xargs)"

if [[ "$formula_url" != "$sha_url" ]]; then
  echo "formula url mismatch: $formula_url" >&2
  exit 2
fi

if [[ "$formula_sha" != "$sha256" ]]; then
  echo "formula sha mismatch: $formula_sha (expected $sha256)" >&2
  exit 2
fi

brew update
brew uninstall gogcli || true
brew untap steipete/tap || true
brew tap steipete/tap
brew install steipete/tap/gogcli
brew test steipete/tap/gogcli
gog --help

rm -f "$notes_file" "$sha_file"

echo "Release v$version verified (CI, GitHub release notes/assets, Homebrew install/test)."
