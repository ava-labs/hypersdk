#!/usr/bin/env bash
# Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

rm -rf ~/.tmpnet

go build -o ./build/o1f99fVdFpnvKTT9gM3dC11Du2t9cNQhRuuDzdF6urYaPbG4M ./cmd/hyperevm

mv ./build/o1f99fVdFpnvKTT9gM3dC11Du2t9cNQhRuuDzdF6urYaPbG4M $AVALANCHEGO_PLUGIN_PATH
