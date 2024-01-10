#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

source ../../scripts/constants.sh
source ../../scripts/common/utils.sh

set -o errexit
set -o nounset
set -o pipefail

set_cgo_flags

check_repository_root scripts/build.release.sh

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
