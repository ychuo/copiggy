#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
. ./scripts/env.sh

if [ "$(uname -s)" != Darwin ]; then
  echo 'Packaging a Copiggy release requires macOS.' >&2
  exit 1
fi
copiggy_version=$(/usr/bin/plutil -extract version raw -o - mygo.json)
case "$copiggy_version" in
  ''|*[!A-Za-z0-9.+_-]*|.|..)
    echo 'The release version must be a nonempty filename-safe version in mygo.json.' >&2
    exit 1
    ;;
esac
copiggy_releases="$PWD/build/releases"
copiggy_final="$copiggy_releases/v$copiggy_version"
if [ -e "$copiggy_final" ] || [ -L "$copiggy_final" ]; then
  printf 'Release v%s already exists at %s. Keep the existing files or increment the version in mygo.json.\n' "$copiggy_version" "$copiggy_final" >&2
  exit 1
fi
copiggy_arch=$(go env GOARCH)
case "$copiggy_arch" in
  arm64|amd64) ;;
  *) printf 'Unsupported macOS release architecture: %s\n' "$copiggy_arch" >&2; exit 1 ;;
esac

mkdir -p build/.staging
copiggy_stage=$(mktemp -d "$PWD/build/.staging/release-XXXXXX")
cleanup() {
  rm -rf "$copiggy_stage"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
copiggy_candidate="$copiggy_stage/v$copiggy_version"
mkdir "$copiggy_candidate"

./scripts/package.sh
copiggy_archive="Copiggy-v$copiggy_version-macos-$copiggy_arch.zip"
/usr/bin/ditto -c -k --sequesterRsrc --keepParent build/Copiggy.app "$copiggy_candidate/$copiggy_archive"
/usr/bin/unzip -tq "$copiggy_candidate/$copiggy_archive"
(
  cd "$copiggy_candidate"
  /usr/bin/shasum -a 256 "$copiggy_archive" > SHA256SUMS.txt
  /usr/bin/shasum -a 256 -c SHA256SUMS.txt
)

# Publish the verified pair together. No-clobber also preserves any release
# created by another packaging process while this build was in progress.
mkdir -p "$copiggy_releases"
mv -n "$copiggy_candidate" "$copiggy_releases/"
if [ -d "$copiggy_candidate" ]; then
  printf 'Release v%s appeared during packaging. Existing files were preserved; increment the version in mygo.json to create another release.\n' "$copiggy_version" >&2
  exit 1
fi
printf 'Release files: %s\n' "$copiggy_final"
