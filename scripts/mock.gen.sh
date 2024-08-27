#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

if ! [[ "$0" =~ scripts/mock.gen.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=/scripts/common/utils.sh
source "$SCRIPT_DIR"/common/utils.sh
# shellcheck source=/scripts/constants.sh
source "$SCRIPT_DIR"/constants.sh

install_if_not_exists mockgen go.uber.org/mock/mockgen@v0.4.0

# tuples of (source interface import path, comma-separated interface names, output file path)
input="scripts/mocks.mockgen.txt"
while IFS= read -r line
do
  IFS='=' read -r src_import_path interface_name output_path <<< "${line}"
  package_name=$(basename "$(dirname "$output_path")")
  echo "Generating ${output_path}..."
  mockgen -package="${package_name}" -destination="${output_path}" "${src_import_path}" "${interface_name}"
done < "$input"

echo "SUCCESS"
