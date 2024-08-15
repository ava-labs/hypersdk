#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -e

if ! [[ "$0" =~ scripts/fix.lint.sh ]]; then
  echo "must be run from hypersdk root"
  exit 255
fi

# Calculate the directory where the script is located
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# Source the utils.sh script using an absolute path
# shellcheck source=/scripts/common/utils.sh
source "${SCRIPT_DIR}/common/utils.sh"

add_license_headers ""

echo "gofumpt files"
go install -v mvdan.cc/gofumpt@latest
gofumpt -l -w .

echo "gci files"
go install -v github.com/daixiang0/gci@v0.12.1
gci write --skip-generated -s standard -s default -s blank -s dot -s "prefix(github.com/ava-labs/hypersdk)" -s alias --custom-order .
