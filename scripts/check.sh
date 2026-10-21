#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
. ./scripts/env.sh

copiggy_goroot=$(go env GOROOT)
unformatted=$("$copiggy_goroot/bin/gofmt" -l main.go internal resources)
if [ -n "$unformatted" ]; then
  printf 'Run gofmt on:\n%s\n' "$unformatted" >&2
  exit 1
fi

# Default tests never use the desktop or real clipboard. Native event callback
# integration checks are separately gated by the integration build tag.
export COPIGGY_NATIVE_SCREENSHOTS=
go test ./... -timeout 120s
# MyGo vet runs go vet before checking native UI element lifetimes.
go tool mygo vet
cmp resources/brand/app-icon.png resources/brand/Copiggy.icon/Assets/pig.png
mkdir -p build
go build -trimpath -tags mygo_noinspector \
  -ldflags '-X github.com/egoist/mygo.production=1' -o build/check/copiggy .
