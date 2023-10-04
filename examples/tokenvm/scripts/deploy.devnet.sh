#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Set constants
export ARCH_TYPE=$(uname -m)
[ $ARCH_TYPE = x86_64 ] && ARCH_TYPE=amd64
echo ARCH_TYPE: ${ARCH_TYPE}
export OS_TYPE=$(uname | tr '[:upper:]' '[:lower:]')
echo OS_TYPE: ${OS_TYPE}

# Check valid setup
if [ ${OS_TYPE} != 'darwin' ]; then
  echo 'os is not supported' >&2
  exit 1
fi
if [ ${ARCH_TYPE} != 'arm64' ]; then
  echo 'arch is not supported' >&2
  exit 1
fi
if ! [ -x "$(command -v aws)" ]; then
  echo 'aws-cli is not installed' >&2
  exit 1
fi

# Install avalanche-ops
rm -f /tmp/avalancheup-aws
wget https://github.com/ava-labs/avalanche-ops/releases/download/latest/avalancheup-aws.aarch64-apple-darwin
mv ./avalancheup-aws.aarch64-apple-darwin /tmp/avalancheup-aws
chmod +x /tmp/avalancheup-aws
/tmp/avalancheup-aws --help

# Install 
