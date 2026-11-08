#!/usr/bin/env bash
# Builds AppCloser.app (Apple silicon + Intel) into dist/ and zips it.
# Runs on macOS (Xcode command line tools).
set -euo pipefail
cd "$(dirname "$0")"

export MACOSX_DEPLOYMENT_TARGET=11.0
export CGO_ENABLED=1
version=$(tr -d '[:space:]' < VERSION)
work=dist/mac-build
app=dist/AppCloser-macOS/AppCloser.app
rm -rf "$work" dist/AppCloser-macOS dist/AppCloser-macOS.zip
mkdir -p "$work" "$app/Contents/MacOS" "$app/Contents/Resources"

for arch in arm64 amd64; do
	GOARCH=$arch go build -trimpath -ldflags="-s -w" -o "$work/AppCloser-$arch" .
	GOARCH=$arch go build -tags gui -trimpath -ldflags="-s -w" -o "$work/AppCloser-Settings-$arch" .
done
lipo -create -output "$app/Contents/MacOS/AppCloser" "$work/AppCloser-arm64" "$work/AppCloser-amd64"
lipo -create -output "$app/Contents/MacOS/AppCloser-Settings" "$work/AppCloser-Settings-arm64" "$work/AppCloser-Settings-amd64"

sed "s/VERSION/$version/g" packaging/macos/Info.plist > "$app/Contents/Info.plist"
go run ./tools/mkicon -iconset "$work/AppCloser.iconset"
iconutil -c icns -o "$app/Contents/Resources/AppCloser.icns" "$work/AppCloser.iconset"

# Ad-hoc signature: required to run on Apple silicon (no paid developer account).
codesign --force --sign - "$app/Contents/MacOS/AppCloser-Settings"
codesign --force --sign - "$app"
codesign --verify --strict "$app"

cp packaging/README-macos.txt dist/AppCloser-macOS/README.txt
rm -rf "$work"
(cd dist && ditto -c -k --keepParent AppCloser-macOS AppCloser-macOS.zip)
echo "dist/AppCloser-macOS.zip"
