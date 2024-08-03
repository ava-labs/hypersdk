#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

go run ./cmd/morpheus-cli/ chain import-anr; 

date=$(date +%Y-%m-%d-%H-%M-%S)

# Run the command and output to both the log file and console
go run ./cmd/morpheus-cli/ spam run ed25519 \
    --accounts=20000 \
    --txs-per-second=10000 \
    --min-capacity=1000 \
    --step-size=50 \
    --s-zipf=1.01 \
    --v-zipf=2.7 \
    --conns-per-host=10 2>&1 | tee "spam-${date}.log"