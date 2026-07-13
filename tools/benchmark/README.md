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
## Benchmark suite (external cluster)
To run multiple benchmark workloads against an **existing** etcd cluster, use
`scripts/benchmark_suite.sh` from the repository root:
```
  $ ./scripts/benchmark_suite.sh --endpoints http://127.0.0.1:2379
  $ ./scripts/benchmark_suite.sh --profile heavy --endpoints http://127.0.0.1:2379
  $ ./scripts/benchmark_suite.sh --config ./my-load.conf --endpoints http://10.0.0.5:2379
```
The script runs `put`, `stm`, `txn-mixed`, `txn-put`, `range`, `lease-keepalive`,
and watch benchmarks. It writes results under the output directory and produces
`summary.csv`.
**Warning:** the suite writes data to the target cluster and may run compact/defrag
before watch workloads. Do not run it against production clusters.
### Configuring load
Load can be configured in four ways (highest to lowest precedence):
1. **CLI flags** — e.g. `--clients 100 --put-total 3000000`
2. **Environment variables** — e.g. `CLIENTS=100 PUT_TOTAL=3000000`
3. **Config file** — `--config path/to/file.conf` (see `scripts/benchmark_suite.conf.example`)
4. **Built-in profiles** — `--profile quick` (default) or `--profile heavy`
Examples:
```
  $ cp scripts/benchmark_suite.conf.example ~/my-etcd-load.conf
  $ ./scripts/benchmark_suite.sh --config ~/my-etcd-load.conf --endpoints http://127.0.0.1:2379
  $ ./scripts/benchmark_suite.sh --operations put,range,watch-latency \
      --clients 50 --put-total 500000 --endpoints http://127.0.0.1:2379
```
Note: `benchmark mvcc put` is not included because it benchmarks local MVCC
storage and does not use cluster endpoints.

## Usage

The following command should output the usage per the latest development.

```
  $ benchmark --help
```
