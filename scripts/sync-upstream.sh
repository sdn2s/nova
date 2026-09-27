#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

src="${1:-.upstream}"
if [[ ! -d "$src" ]]; then
  git clone --depth 1 https://github.com/Flowseal/zapret-discord-youtube.git "$src"
elif [[ -z "${1:-}" ]]; then
  git -C "$src" pull --ff-only
fi
echo "upstream: $(cat "$src/.service/version.txt") ($(git -C "$src" rev-parse --short HEAD))"

mkdir -p bin
cp "$src"/bin/* bin/
(cd bin && shasum -a 256 $(ls | grep -v SHA256SUMS) > SHA256SUMS)

mkdir -p lists
for f in list-general.txt list-google.txt list-exclude.txt ipset-exclude.txt; do
  cp "$src/lists/$f" lists/
done
cp "$src/.service/ipset-service.txt" lists/ipset-all.txt

for f in list-general-user.txt list-exclude-user.txt ipset-exclude-user.txt; do
  [[ -f "lists/$f" ]] || printf '# one entry per line; lines starting with # are ignored\n' > "lists/$f"
done

rm -f internal/batimport/testdata/upstream/*.bat
cp "$src"/general*.bat internal/batimport/testdata/upstream/
go run ./cmd/batimport -src "$src" -out strategies
go test ./...
