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

echo "adding license header"
go install -v github.com/palantir/go-license@latest

# alert the user if they do not have $GOPATH properly configured
check_command go-license

go-license --config="${SCRIPT_DIR}/../license.yml" -- **/*.go

echo "gofumpt files"
go install mvdan.cc/gofumpt@latest
gofumpt -l -w .
