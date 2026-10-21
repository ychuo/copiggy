#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
./scripts/package.sh
exec /usr/bin/open "$PWD/build/Copiggy.app"
