#!/bin/bash

set -exu

go run ./cmd/morpheus-cli/ chain import-anr

go run ./cmd/morpheus-cli/ spam run ed25519 \
    --accounts=20000 \
    --txs-per-second=10000 \
    --min-capacity=1000 \
    --step-size=50 \
    --s-zipf=1.01 \
    --v-zipf=2.7 \
    --conns-per-host=10