# Source from the project root. All entry points share the toolchain and caches.
if ! command -v go >/dev/null 2>&1 && [ -x "$PWD/.cache/toolchains/go/bin/go" ]; then
  export PATH="$PWD/.cache/toolchains/go/bin:$PATH"
fi
export GOPATH="$PWD/.cache/go"
export GOCACHE="$PWD/.cache/go-build"
export GOTOOLCHAIN=go1.27.1
export CGO_ENABLED=0
# Source archives without Git metadata are valid build inputs.
export GOFLAGS="${GOFLAGS:-} -buildvcs=false"
# Production packaging must never inherit the inspector opt-in.
export MYGO_INSPECTOR=0
