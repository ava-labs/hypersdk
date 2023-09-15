#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o nounset
set -o pipefail

# Remove any previous build artifacts
rm -rf token-wallet.zip

# Build file for local arch
#
# Don't use upx: https://github.com/upx/upx/issues/446
wails build -clean -platform darwin/universal
codesign -s ${APP_SIGNING_KEY_ID} --deep  --timestamp -o runtime -v build/bin/Token\ Wallet.app
ditto -c -k --keepParent build/bin/Token\ Wallet.app token-wallet.zip

# Need to sign to allow for app to be opened on other computers
xcrun altool --notarize-app --primary-bundle-id com.ava-labs.token-wallet --username ${APPLE_NOTARIZATION_USERNAME} --password "@keychain:altool" --file token-wallet.zip

# Log until exit
read -p "Input APPLE_NOTARIZATION_REQUEST_ID: " APPLE_NOTARIZATION_REQUEST_ID
while true
do
  xcrun altool --notarization-info ${APPLE_NOTARIZATION_REQUEST_ID} -u ${APPLE_NOTARIZATION_USERNAME} -p "@keychain:altool"
  sleep 15
done
