#!/bin/bash

set -exu -o pipefail

# Get the absolute path of the script directory
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# Kill existing processes
pkill -9 -f avalanchego || true
pkill -9 -f spam || true

# Clean up and restart
rm -rf ~/.tmpnet/ 
cd "$SCRIPT_DIR/examples/morpheusvm/"
./scripts/run.sh
go run ./cmd/morpheus-cli/ spam run ed25519
