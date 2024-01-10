#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

source ../../scripts/common/utils.sh
source ../../scripts/constants.sh

set -o errexit
set -o nounset
set -o pipefail

set_cgo_flags

# Install wails
go install -v github.com/wailsapp/wails/v2/cmd/wails@v2.5.1

# alert the user if they do not have $GOPATH properly configured
check_command wails

# Start development environment
wails dev
