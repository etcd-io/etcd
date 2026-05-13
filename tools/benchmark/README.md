# etcd/tools/benchmark

`etcd/tools/benchmark` is the official benchmarking tool for etcd clusters.

## Installation

Install the tool by running the following command from the etcd source directory.

```
  $ go install -v ./tools/benchmark
```

The installation will place executables in the $GOPATH/bin. If $GOPATH environment variable is not set, the tool will be installed into the $HOME/go/bin. You can also find out the installed location by running the following command from the etcd source directory. Make sure that $PATH is set accordingly in your environment.

```
  $ go list -f "{{.Target}}" ./tools/benchmark
```

Alternatively, instead of installing the tool, you can use it by simply running the following command from the etcd source directory.

```
  $ go run ./tools/benchmark
```

## Usage

The following command should output the usage per the latest development.

```
  $ benchmark --help
```

## Run Metrics Capture

Benchmark invocations can capture a per-run JSON artifact with sampled etcd
member metrics and local host OS metrics.

Enable capture with:

```
  $ benchmark --capture-run-metrics ...
```

Useful flags:

```
  --capture-run-metrics
  --metrics-urls=http://127.0.0.1:2381/metrics,http://127.0.0.1:2383/metrics
  --metrics-sample-interval=5s
  --metrics-step-name=clients-512
  --metrics-run-group=lwi-put-ramp
  --metrics-output=./_artifacts/clients-512.json
  --metrics-elbow-graph-output=./_artifacts/lwi-put-ramp-elbow.html
  --metrics-summary-table-output=./_artifacts/lwi-put-ramp-summary.md
```

If `--metrics-urls` is omitted, the tool derives member `/metrics` URLs from
`--endpoints`. Each benchmark invocation is treated as a single run or ramp
step, and `--metrics-step-name` can be used to label that step in the artifact.
When the command exits, `benchmark` prints the sampled run metrics summary to
stdout followed by the artifact path.

When `--metrics-elbow-graph-output` is set, the command also writes or updates
an HTML graph from the run metrics artifacts in the same artifact directory. Use
the same `--metrics-run-group` for every ramp step to keep unrelated runs out of
the graph. The graph marks the first step where benchmark P99 doubles from the
previous step or member WAL fsync P99 crosses 25ms.

When `--metrics-summary-table-output` is set, the command also writes or updates
a Markdown table for the same ramp. The table includes input parameter columns
for the command-line flags used in the run, the benchmark latency distribution
for each recorded operation, and the etcd and host metrics needed for the
capacity report.

The captured report includes per-member samples and summaries for:

- `etcd_disk_wal_fsync_duration_seconds` P50, P90, P99, P999
- `etcd_disk_backend_commit_duration_seconds` P50, P90, P99, P999
- `etcd_server_proposals_pending`
- `etcd_server_leader_changes_seen_total`
- `etcd_mvcc_db_total_size_in_bytes`

The report also includes local host CPU, memory, disk I/O, and network I/O
samples from the machine running `benchmark`. It does not collect OS metrics
from remote etcd member hosts.

## Scenario Mapping

The benchmark tool can be used to exercise the main management-plane scenarios
that matter for LWI-style orchestration loads:

- Sustained write throughput: `benchmark put --rate=<writes/sec> --total=<writes>`
- CAS phase transitions: `benchmark txn-put`
  This benchmark now uses `Txn(Compare(ModRevision)=X, Then(Put))` semantics so
  it matches optimistic 2PC-style state transitions instead of unconditional
  txn writes.
- Watch fan-out and put-to-delivery latency: `benchmark watch-latency --put-rate=<writes/sec> --put-total=<writes> --streams=<watch streams> --watchers-per-stream=<watchers per stream>`
- Mixed writes + watchers + range reads: `benchmark watch-latency --put-rate=<writes/sec> --put-total=<writes> --streams=<watch streams> --watchers-per-stream=<watchers per stream> --range-total=<reads> --range-rate=<reads/sec>`

`--streams * --watchers-per-stream` is the total concurrent watcher count.

The `watch-latency` command records:

- put latency for the writes it issues
- watch delivery latency from the end of each put to each watcher receiving the event
- optional range-read latency when `--range-total` is greater than zero

## Ramp Guidance

To find a breaking point instead of a smoke-test point, run the same scenario at
increasing write rates with long enough totals to sustain the step duration you
care about. For example, a 5-minute step at `1000` writes/sec would use
`--rate=1000 --total=300000`.

For a gentle KILT cluster smoke ramp, start with short, low-rate steps and only
increase after checking the generated summary for WAL fsync, pending proposals,
DB growth, network bytes, and errors:

```
  $ benchmark put \
      --rate=25 \
      --total=750 \
      --clients=4 \
      --conns=2 \
      --key-space-size=10000 \
      --capture-run-metrics \
      --metrics-sample-interval=5s \
      --metrics-step-name=rate-25 \
      --metrics-run-group=kilt-gentle-put \
      --metrics-output=./_artifacts/kilt-gentle-put-rate-25.json \
      --metrics-elbow-graph-output=./_artifacts/kilt-gentle-put-elbow.html \
      --metrics-summary-table-output=./_artifacts/kilt-gentle-put-summary.md
```

The `--capture-run-metrics` flags are intended to be used for each ramp step so
the benchmark latency summaries can be compared against the sampled member and
host metrics artifact for the same run.

For a ramp, pass a stable `--metrics-run-group` and the same
`--metrics-elbow-graph-output` and `--metrics-summary-table-output` on each
step. The graph and table are regenerated after each step, so after the final
run they contain the full ramp.

## Limits

If all etcd members are placed on one machine sharing one disk, the results are
best interpreted as a software breaking point under shared-disk contention.

That setup does not answer real-cluster capacity planning questions well,
because it removes inter-node network latency while also introducing unrealistic
shared fsync contention across members.
