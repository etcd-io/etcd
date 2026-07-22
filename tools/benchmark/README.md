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

## Running via make (local etcd)

From the repository root, use the `bench-*` make targets. Each target builds etcd,
installs the benchmark tool if needed, starts a temporary local etcd process, runs
the named workload with `--report-perfdash`, then tears the server down
(see `scripts/benchmark_test.sh` and [#16467](https://github.com/etcd-io/etcd/issues/16467)).

```
  $ make bench-put
  $ make bench-range
  $ make bench-txn-put
  $ make bench-stm
  $ make bench-lease-keepalive
  $ make bench-watch
  $ make bench-watch-latency
```

Pass extra flags through `ARGS`:

```
  $ make bench-put ARGS='--total=100000 --clients=100 --conns=10'
  $ make bench-range ARGS='--total=50000 --consistency=s --limit=100'
```

`bench-range` uses a default key of `foo`. For custom range bounds, call the
script directly:

```
  $ ./scripts/benchmark_test.sh range mykey myend --total=10000
```

## Usage

The following command should output the usage per the latest development.

```
  $ benchmark --help
```
