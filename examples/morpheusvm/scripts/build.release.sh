#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o nounset
set -o pipefail

if [ -n "$1" ]; then
    # Attempt to change to the directory provided as the argument
    if cd "$1"; then
        echo "cd $1"
    else
        echo "failed to cd to directory; $1"
        exit 255
    fi
fi

# shellcheck source=/scripts/constants.sh
source ../../scripts/constants.sh
# shellcheck source=/scripts/common/utils.sh
source ../../scripts/common/utils.sh

# https://goreleaser.com/install/
go install -v github.com/goreleaser/goreleaser@latest

# alert the user if they do not have $GOPATH properly configured
check_command goreleaser

# e.g.,
# git tag 1.0.0
goreleaser release \
--config .goreleaser.yml \
--skip-announce \
--skip-publish
