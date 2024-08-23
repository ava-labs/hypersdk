#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o pipefail
set -e

if ! [[ "$0" =~ scripts/fix.lint.sh ]]; then
  echo "must be run from hypersdk root"
  exit 255
fi

# Calculate the directory where the script is located
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=/scripts/common/utils.sh
source "${SCRIPT_DIR}/common/utils.sh"

add_license_headers ""

echo "gofumpt files"
install_if_not_exists gofumpt mvdan.cc/gofumpt@latest
gofumpt -l -w .

echo "gci files"
install_if_not_exists gci github.com/daixiang0/gci@v0.12.1
gci write --skip-generated -s standard -s default -s blank -s dot -s "prefix(github.com/ava-labs/hypersdk)" -s alias --custom-order .
