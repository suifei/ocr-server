#!/bin/sh
# Build release archives from a clean, committed tree; no upload or tagging.
set -eu
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 vMAJOR.MINOR.PATCH NEW_OUTPUT_DIRECTORY" >&2
    exit 2
fi
tag=$1
printf '%s\n' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || exit 2
case $2 in
    /*) out=$2 ;;
    *)  out=$PWD/$2 ;;
esac
cd "$(dirname "$0")/.."
if [ -n "$(git status --porcelain)" ]; then
    echo "Build from a clean checkout." >&2
    exit 1
fi
# Require a fresh output directory and do not overwrite existing artifacts.
mkdir "$out"
source=$(git rev-parse HEAD)
toolchain=$(go version)
stage=""
trap '[ -z "$stage" ] || rm -rf -- "$stage"' EXIT
trap 'exit 1' HUP INT TERM
for platform in linux-amd64 macos-arm64 windows-amd64; do
    target=${platform%-*}
    arch=${platform##*-}
    executable=ocr-server
    case $target in
        macos) target=darwin ;;
        windows) executable=ocr-server.exe ;;
    esac
    stage=$(mktemp -d)
    mkdir -p "$stage/test" "$stage/docs"
    GOOS=$target GOARCH=$arch CGO_ENABLED=0 go build -trimpath -buildvcs=true \
        -ldflags "-X main.version=${tag#v}" -o "$stage/$executable" ./cmd/server
    cp README.md LICENSE requirements-cpu.txt "$stage/"
    cp docs/deploy.md "$stage/docs/"
    cp test/smoke_local.py test/ocr_fixture.png "$stage/test/"
    printf 'Source: %s\nGo: %s\nPlatform: %s\nVersion: %s\nPython and models are separate.\n' \
        "$source" "$toolchain" "$platform" "${tag#v}" > "$stage/BUILD.txt"
    (cd "$stage" && zip -q -X -r "$out/ocr-server-$tag-$platform.zip" \
        "$executable" README.md LICENSE requirements-cpu.txt BUILD.txt docs test)
    rm -rf -- "$stage"
    stage=""
done
(cd "$out" && {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum ./*.zip
    else
        shasum -a 256 ./*.zip
    fi
} | sed 's|  \./|  |' > SHA256SUMS.txt)
