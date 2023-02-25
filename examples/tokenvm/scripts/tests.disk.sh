#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o pipefail
set -e

if ! [[ "$0" =~ scripts/tests.disk.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# You need to install `fio` to run this job

if ! command -v ginkgo &> /dev/null; then
  echo "installing fio"
  if [[ $OSTYPE == 'darwin'* ]]; then
    echo 'macOS detected'
    brew install -y fio
  else
    # Assume some variant of linux
    sudo apt update
    sudo apt install -y fio
  fi
else
  echo "fio already installed"
fi

# This testing approach was inspired by this article:
# https://cloud.google.com/compute/docs/disks/benchmarking-pd-performance
echo -e "\033[0;32mtesting write throughput\033[0m"
rm -rf tmp-storage-testing
mkdir -p tmp-storage-testing
fio --name=write_throughput --directory=tmp-storage-testing --numjobs=5 \
--size=10G --time_based --runtime=30s --ramp_time=2s \
--direct=1 --verify=0 --bs=1M --iodepth=64 --rw=write \
--group_reporting=1

echo -e "\033[0;32mtesting write iops\033[0m"
rm -rf tmp-storage-testing
mkdir -p tmp-storage-testing
fio --name=write_iops --directory=tmp-storage-testing --size=10G \
--time_based --runtime=30s --ramp_time=2s --direct=1 \
--verify=0 --bs=4K --iodepth=256 --rw=randwrite --group_reporting=1

echo -e "\033[0;32mtesting read throughput\033[0m"
rm -rf tmp-storage-testing
mkdir -p tmp-storage-testing
fio --name=read_throughput --directory=tmp-storage-testing --numjobs=5 \
--size=10G --time_based --runtime=30s --ramp_time=2s \
--direct=1 --verify=0 --bs=1M --iodepth=64 --rw=read \
--group_reporting=1

echo -e "\033[0;32mtesting read iops\033[0m"
rm -rf tmp-storage-testing
mkdir -p tmp-storage-testing
fio --name=read_iops --directory=tmp-storage-testing --size=10G \
--time_based --runtime=30s --ramp_time=2s --direct=1 \
--verify=0 --bs=4K --iodepth=256 --rw=randread --group_reporting=1

echo -e "\033[0;32mcleaning up...\033[0m"
rm -rf tmp-storage-testing
