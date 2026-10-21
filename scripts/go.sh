#!/bin/sh
# Keep the native build and tests on the same toolchain and local caches.
set -eu
cd "$(dirname "$0")/.."
. ./scripts/env.sh
exec go "$@"
