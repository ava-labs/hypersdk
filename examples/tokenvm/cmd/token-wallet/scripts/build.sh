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

PUBLISH=${PUBLISH:-true}

# Install wails
go install -v github.com/wailsapp/wails/v2/cmd/wails@v2.5.1

# alert the user if they do not have $GOPATH properly configured
check_command wails

# Build file for local arch
#
# Don't use upx: https://github.com/upx/upx/issues/446
wails build -clean -platform darwin/universal

OUTPUT=build/bin/Token\ Wallet.app
if [ ! -d "$OUTPUT" ]; then
  exit 1
fi

# Remove any previous build artifacts
rm -rf token-wallet.zip

# Exit early if not publishing
if [ "${PUBLISH}" == false ]; then
  echo "not publishing app"
  ditto -c -k --keepParent build/bin/Token\ Wallet.app token-wallet.zip
  exit 0
fi
echo "publishing app"

# Sign code
codesign -s "${APP_SIGNING_KEY_ID}" --deep  --timestamp -o runtime -v build/bin/Token\ Wallet.app
ditto -c -k --keepParent build/bin/Token\ Wallet.app token-wallet.zip

# Need to sign to allow for app to be opened on other computers
xcrun altool --notarize-app --primary-bundle-id com.ava-labs.token-wallet --username "${APPLE_NOTARIZATION_USERNAME}" --password "@keychain:altool" --file token-wallet.zip

# Log until exit
read -r -p "Input APPLE_NOTARIZATION_REQUEST_ID: " APPLE_NOTARIZATION_REQUEST_ID
while true
do
  xcrun altool --notarization-info "${APPLE_NOTARIZATION_REQUEST_ID}" -u "${APPLE_NOTARIZATION_USERNAME}" -p "@keychain:altool"
  sleep 15
done
