#!/usr/bin/env bash
# Copyright 2026 The etcd Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script runs a benchmark on a locally started etcd server

set -euo pipefail

source ./scripts/test_lib.sh

ETCD_ARGS="--backend-batch-interval=100ms --backend-batch-limit=1000" ./scripts/benchmark_test.sh put --clients 100 --conns 10 --key-size 128 --val-size 4000 --total 1000000 --key-space-size 100000 --rate 4000 --compact-interval 1m --sequential-keys

# etcd log location is /tmp/etcd/etcd.log
# Extract the "took" value from the JSON log entry
took_value=$(grep "finished scheduled compaction" /tmp/etcd.log | tail -1 | grep -o '"took":"[^"]*"' | cut -d'"' -f4)
log_callout "Compaction Time: $took_value"

# Read first file in _artifacts directory and extract "Perc99" value
first_artifact=$(ls -t _artifacts/*.json | head -1)
perc99_value=$(cat "$first_artifact" | grep "Perc99" | awk -F': ' '{print $2}' | tr -d ',')
log_callout "Put Latency Perc99: $perc99_value"
