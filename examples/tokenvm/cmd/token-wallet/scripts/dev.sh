#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o nounset
set -o pipefail

# Get the directory of the script, even if sourced from another directory
SCRIPT_DIR=$(
  cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd
)

# shellcheck source=/scripts/common/utils.sh
source "$SCRIPT_DIR"/../../../../../scripts/common/utils.sh
# shellcheck source=/scripts/constants.sh
source "$SCRIPT_DIR"/../../../../../scripts/constants.sh

# Install wails
go install -v github.com/wailsapp/wails/v2/cmd/wails@v2.5.1

# alert the user if they do not have $GOPATH properly configured
check_command wails

# Start development environment
wails dev
