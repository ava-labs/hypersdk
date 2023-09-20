#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o nounset
set -o pipefail

echo "installing proxy"
go install -v github.com/sensiblecodeio/tiny-ssl-reverse-proxy

# TODO: add ability to provide a cert
echo "starting proxy for ${VM_API} on :9090"
tiny-ssl-reverse-proxy -listen=":9090" -tls=false -where="${VM_API}"
