#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
. ./scripts/env.sh

if [ "$(uname -s)" != Darwin ]; then
  echo 'Packaging Copiggy requires macOS.' >&2
  exit 1
fi
# Use a full Xcode only within this process; leave xcode-select unchanged.
if ! xcrun --find actool >/dev/null 2>&1; then
  if [ -z "${DEVELOPER_DIR:-}" ] && [ -d /Applications/Xcode.app/Contents/Developer ]; then
    export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
  fi
fi
if ! xcrun --find actool >/dev/null 2>&1; then
  echo 'The Copiggy icon requires Xcode 26 or later. Set DEVELOPER_DIR to its Developer directory.' >&2
  exit 1
fi

# The Icon Composer layer is an exact copy of the approved brand artwork.
if ! cmp -s resources/brand/app-icon.png resources/brand/Copiggy.icon/Assets/pig.png; then
  echo 'The Icon Composer pig.png must match resources/brand/app-icon.png.' >&2
  exit 1
fi
copiggy_name=$(/usr/bin/plutil -extract name raw -o - mygo.json)
copiggy_identifier=$(/usr/bin/plutil -extract identifier raw -o - mygo.json)
copiggy_version=$(/usr/bin/plutil -extract version raw -o - mygo.json)
copiggy_build=$(/usr/bin/plutil -extract macos.infoPlist.CFBundleVersion raw -o - mygo.json)
copiggy_minimum=$(/usr/bin/plutil -extract macos.minimumSystemVersion raw -o - mygo.json)
copiggy_signing=$(/usr/bin/plutil -extract macos.signingIdentity raw -o - mygo.json)
test "$copiggy_name" = Copiggy
test "$copiggy_identifier" = dev.copiggy

mkdir -p build
copiggy_stage=$(mktemp -d "$PWD/build/.package-XXXXXX")
copiggy_final="$PWD/build/Copiggy.app"
copiggy_previous="$copiggy_stage/previous.app"
cleanup() {
  if [ -d "$copiggy_previous" ] && [ ! -e "$copiggy_final" ]; then
    if ! mv "$copiggy_previous" "$copiggy_final"; then
      printf 'Previous bundle preserved at %s\n' "$copiggy_previous" >&2
      return
    fi
  fi
  rm -rf "$copiggy_stage"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

copiggy_arch=$(go env GOARCH)
# The pinned CLI provides production=1 and mygo_noinspector. env.sh disables
# the environment override that would otherwise retain the inspector.
go tool mygo build -platform "darwin/$copiggy_arch" -skip-dmg -skip-notarize -o "$copiggy_stage"
copiggy_bundle="$copiggy_stage/darwin-$copiggy_arch/Copiggy.app"
copiggy_plist="$copiggy_bundle/Contents/Info.plist"
copiggy_resources="$copiggy_bundle/Contents/Resources"
copiggy_buildinfo=$(go version -m "$copiggy_bundle/Contents/MacOS/Copiggy")
# Go omits linker flags from trimpath build metadata; the pinned production
# CLI supplies production=1. The inspector tag remains independently visible.
case "$copiggy_buildinfo" in
  *mygo_noinspector*) ;;
  *) echo 'The bundle must be built without the native inspector.' >&2; exit 1 ;;
esac

# MyGo allows PNG fallback when actool fails. Our official bundle requires
# the modern catalog and its same-source ICNS fallback to both be present.
test -s "$copiggy_resources/Assets.car"
test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIconName' "$copiggy_plist")" = AppIcon
copiggy_icns=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIconFile' "$copiggy_plist")
test -s "$copiggy_resources/$copiggy_icns"
test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$copiggy_plist")" = "$copiggy_identifier"
test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$copiggy_plist")" = "$copiggy_version"
test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$copiggy_plist")" = "$copiggy_build"
test "$(/usr/libexec/PlistBuddy -c 'Print :LSMinimumSystemVersion' "$copiggy_plist")" = "$copiggy_minimum"
test "$(/usr/libexec/PlistBuddy -c 'Print :LSUIElement' "$copiggy_plist")" = true

# MyGo derives the executable name from the display name. Adapt its generated
# bundle together with the plist, then sign the complete result again.
mv "$copiggy_bundle/Contents/MacOS/Copiggy" "$copiggy_bundle/Contents/MacOS/copiggy"
/usr/libexec/PlistBuddy -c 'Set :CFBundleExecutable copiggy' "$copiggy_plist"

# Runtime artwork and catalogs are embedded by resources/resources.go. Keep
# their licenses, but do not ship duplicate assets or Icon Composer sources.
mkdir -p "$copiggy_resources/licenses"
cp resources/icons/LICENSE "$copiggy_resources/licenses/Phosphor.txt"
copiggy_mygo=$(go list -m -f '{{.Dir}}' github.com/egoist/mygo)
cp "$copiggy_mygo/LICENSE" "$copiggy_resources/licenses/MyGo.txt"
rm -rf "$copiggy_resources/brand" "$copiggy_resources/icons" "$copiggy_resources/locales"
rm -f "$copiggy_resources/resources.go"
/usr/bin/plutil -lint "$copiggy_plist"
/usr/bin/codesign --force --deep --sign "$copiggy_signing" "$copiggy_bundle"
/usr/bin/codesign --verify --deep --strict "$copiggy_bundle"

# Publish only after every check passes. Restore the old bundle if replacing
# it fails; compilation and icon errors never touch the existing application.
if [ -e "$copiggy_final" ]; then
  mv "$copiggy_final" "$copiggy_previous"
fi
mv "$copiggy_bundle" "$copiggy_final"
printf 'Built %s\n' "$copiggy_final"
