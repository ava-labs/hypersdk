#!/usr/bin/env bash

set -euo pipefail

if ! [[ "$0" =~ scripts/proto_codegen.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

PROTOC_VERSION="27.1"
if [[ $(protoc --version | cut -f2 -d' ') != "${PROTOC_VERSION}" ]]; then
  # e.g., protoc 27.1
  echo "could not find protoc ${PROTOC_VERSION}, is it installed + in PATH?"
  exit 255
fi

PROTOC_GEN_GO_VERSION="v1.28.0"
go install -v google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION}
if [[ $(protoc-gen-go --version | cut -f2 -d' ') != "${PROTOC_GEN_GO_VERSION}" ]]; then
  # e.g., protoc-gen-go v1.28.0
  echo "could not find protoc-gen-go ${PROTOC_GEN_GO_VERSION}, is it installed + in PATH?"
  exit 255
fi

PROTOC_GEN_GO_GRPC_VERSION="1.1.0"
go install -v google.golang.org/grpc/cmd/protoc-gen-go-grpc@v${PROTOC_GEN_GO_GRPC_VERSION}
if [[ $(protoc-gen-go-grpc --version | cut -f2 -d' ') != "${PROTOC_GEN_GO_GRPC_VERSION}" ]]; then
  # e.g., protoc-gen-go-grpc 1.1.0
  echo "could not find protoc-gen-go-grpc ${PROTOC_GEN_GO_GRPC_VERSION}, is it installed + in PATH?"
  exit 255
fi

TARGET=$PWD/proto
if [ -n "${1:-}" ]; then
  TARGET="$1"
fi

cd "$TARGET"

echo "Creating .pb.go files"
protoc --go_out=. --go-grpc_out=. subscriber.proto