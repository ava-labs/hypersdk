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
# shellcheck source=/scripts/common/utils.sh
source "$HYPERSDK_PATH"/scripts/common/utils.sh
# shellcheck source=/scripts/constants.sh
source "$HYPERSDK_PATH"/scripts/constants.sh

if ! command -v mockgen &> /dev/null
then
  echo "mockgen not found, installing..."
  # https://github.com/uber-go/mock
  go install -v go.uber.org/mock/mockgen@v0.4.0
fi

# alert the user if they do not have $GOPATH properly configured
check_command mockgen

if ! command -v hawkeye &> /dev/null
then
  echo "hawkeye not found, installing..."
  cargo install hawkeye
fi

# tuples of (source interface import path, comma-separated interface names, output file path)
input="scripts/mocks.mockgen.txt"
while IFS= read -r line
do
  IFS='=' read -r src_import_path interface_name output_path <<< "${line}"
  package_name=$(basename "$(dirname "$output_path")")
  echo "Generating ${output_path}..."
  mockgen -package="${package_name}" -destination="${output_path}" "${src_import_path}" "${interface_name}"
done < "$input"

# add license headers to mock files
# ignore errors since mockgen files give warnings when edited
hawkeye format || true

echo "SUCCESS"
