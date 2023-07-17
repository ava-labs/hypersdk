#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e


if ! [[ "$0" =~ scripts/mock.gen.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

HYPERSDK_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)
source "$HYPERSDK_PATH"/scripts/constants.sh

if ! command -v mockgen &> /dev/null
then
  echo "mockgen not found, installing..."
  # https://github.com/golang/mock
  go install -v github.com/golang/mock/mockgen@v1.6.0
fi

if ! command -v go-license &> /dev/null
then
  echo "go-license not found, installing..."
  # https://github.com/palantir/go-license
  go install -v github.com/palantir/go-license@latest
fi

# tuples of (source interface import path, comma-separated interface names, output file path)
input="scripts/mocks.mockgen.txt"
while IFS= read -r line
do
  IFS='=' read src_import_path interface_name output_path <<< "${line}"
  package_name=$(basename $(dirname $output_path))
  echo "Generating ${output_path}..."
  mockgen -package=${package_name} -destination=${output_path} ${src_import_path} ${interface_name}

  go-license \
  --config=./license.yml \
  "${output_path}"
done < "$input"

echo "SUCCESS"
