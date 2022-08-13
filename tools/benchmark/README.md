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
