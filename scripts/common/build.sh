# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o nounset
set -o pipefail

HYPERSDK_PATH=$(
    cd "$(dirname "${BASH_SOURCE[0]}")"
    cd .. && pwd
)
source $HYPERSDK_PATH/constants.sh

set_cgo_flags

realpath() {
    [[ $1 = /* ]] && echo "$1" || echo "$PWD/${1#./}"
}

build_project() {
    PROJECT_PATH=$1
    PROJECT_NAME=$2
    
    if [[ $# -eq 3 ]]; then
        BINARY_PATH=$(realpath build/$3)
    else
        # Set default binary directory location
        BINARY_PATH=$PROJECT_PATH/build/$PROJECT_NAME
    fi

    cd $PROJECT_PATH
    echo "Building $PROJECT_NAME in $BINARY_PATH"
    mkdir -p $(dirname $BINARY_PATH)

    # Change into the cmd directory before building
    go build -o $BINARY_PATH ./cmd/$PROJECT_NAME
}
