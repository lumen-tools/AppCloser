#!/usr/bin/env bash
# Builds AppCloser for Linux and Windows (amd64) into dist/ and zips them.
set -euo pipefail
cd "$(dirname "$0")"

flags=(-trimpath -ldflags="-s -w")
winflags=(-trimpath -ldflags="-s -w -H windowsgui")
rm -rf dist && mkdir -p dist/AppCloser-Linux dist/AppCloser-Windows

CGO_ENABLED=0 go build "${flags[@]}" -o dist/AppCloser-Linux/AppCloser .
go build -tags gui "${flags[@]}" -o dist/AppCloser-Linux/AppCloser-Settings .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build "${winflags[@]}" -o dist/AppCloser-Windows/AppCloser.exe .
CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 \
	go build -tags gui "${winflags[@]}" -o dist/AppCloser-Windows/AppCloser-Settings.exe .

cp packaging/README-linux.txt dist/AppCloser-Linux/README.txt
cp packaging/README-windows.txt dist/AppCloser-Windows/README.txt
(cd dist && zip -qr AppCloser-Linux.zip AppCloser-Linux && zip -qr AppCloser-Windows.zip AppCloser-Windows)
echo "dist/AppCloser-Linux.zip, dist/AppCloser-Windows.zip"
