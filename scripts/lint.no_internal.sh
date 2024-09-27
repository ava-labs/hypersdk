#!/bin/bash

# Find all Go files in the /examples directory
violations=0

if ! [[ "$0" =~ scripts/lint.no_internal.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
EXAMPLES_DIR="${SCRIPT_DIR}/../examples"

for file in $(find "$EXAMPLES_DIR" -type f -name "*.go"); do
  if grep -q '"github.com/ava-labs/hypersdk/internal' "$file"; then
    abs_file=$(realpath "$file")
    clean_file=${abs_file/"$SCRIPT_DIR\/.."/"$SCRIPT_DIR"}
    
    echo "Violation: $clean_file imports from /internal"
    violations=$((violations+1))
  fi
done

if [ $violations -gt 0 ]; then
  exit 1
else
  echo "Ok."
fi
