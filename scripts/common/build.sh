# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o nounset
set -o pipefail

realpath() {
    [[ $1 = /* ]] && echo "$1" || echo "$PWD/${1#./}"
}

build_project() {
    local project_path=$1
    local project_name=$2

    local binary_path
    if [[ $# -eq 3 ]]; then
        local binary_dir=$(cd "$(dirname "$3")" && pwd)
        local binary_name=$(basename "$3")
        binary_path=$binary_dir/$binary_name
    else
        # Set default binary directory location
        binary_path=$project_path/build/$project_name
    fi

    cd "$project_path"
    echo "Building $project_name in $binary_path"
    mkdir -p "$(dirname "$binary_path")"

    go build -o "$binary_path" ./cmd/"$project_name"
}
