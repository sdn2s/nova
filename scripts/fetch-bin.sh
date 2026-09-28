#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

ref="$(cat upstream.ref)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

git init -q "$tmp"
git -C "$tmp" fetch -q --depth 1 https://github.com/Flowseal/zapret-discord-youtube.git "$ref"
git -C "$tmp" checkout -q FETCH_HEAD

mkdir -p bin
cp "$tmp"/bin/* bin/
cp bin.sha256 bin/SHA256SUMS
(cd bin && shasum -a 256 -c SHA256SUMS)
