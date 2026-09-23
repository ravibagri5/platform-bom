#!/bin/sh
# Build helper; avoids interactive shell quirks with ./... globs.
set -e
cd "$(dirname "$0")/.."
export GOTOOLCHAIN=local
case "${1:-build}" in
  tidy) go mod tidy ;;
  build) go build -ldflags "-X github.com/ravibagri5/platform-bom/internal/cli.Version=${VERSION:-dev}" -o bin/pbom ./cmd/pbom ;;
  vet) go vet ./... ;;
  test) go test ./... ;;
  all) go mod tidy && go vet ./... && go test ./... && go build -o bin/pbom ./cmd/pbom ;;
esac
