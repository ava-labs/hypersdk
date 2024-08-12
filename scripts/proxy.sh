#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o nounset
set -o pipefail

HYPERSDK_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)
# shellcheck source=/scripts/common/utils.sh
source "$HYPERSDK_PATH"/scripts/common/utils.sh

echo "installing proxy"
go install -v github.com/sensiblecodeio/tiny-ssl-reverse-proxy

# alert the user if they do not have $GOPATH properly configured
check_command tiny-ssl-reverse-proxy

# TODO: add ability to provide a cert
echo "starting proxy for ${VM_API} on :9090"
tiny-ssl-reverse-proxy -listen=":9090" -tls=false -where="${VM_API}"
