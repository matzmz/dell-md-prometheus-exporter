#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
GO_IMAGE=${GO_IMAGE:-golang:1.25}
OUTPUT_NAME=${OUTPUT_NAME:-dell-md-exporter}

docker run --rm \
  -v "${ROOT_DIR}:/workspace" \
  -w /workspace \
  --entrypoint /bin/sh \
  -e OUTPUT_NAME="${OUTPUT_NAME}" \
  "${GO_IMAGE}" \
  -c 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 /usr/local/go/bin/go build -buildvcs=false -o "/workspace/${OUTPUT_NAME}" ./cmd/dell_md_exporter'
