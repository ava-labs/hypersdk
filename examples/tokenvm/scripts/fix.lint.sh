#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.


set -o errexit
set -o pipefail
set -e

if ! [[ "$0" =~ scripts/fix.lint.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

echo "adding license header"
go install -v github.com/palantir/go-license@latest
files=()
while IFS= read -r line; do files+=("$line"); done < <(find . -type f -name '*.go' ! -name '*.pb.go' ! -name 'mock_*.go')
go-license --config=./license.yml "${files[@]}"

echo "gofumpt files"
go install mvdan.cc/gofumpt@latest
gofumpt -l -w .
