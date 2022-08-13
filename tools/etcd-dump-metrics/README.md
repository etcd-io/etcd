# etcd-dump-metrics

 `etcd-dump-metrics` provides metrics for the latest main branch, a given endpoint, or version.

## Installation

Install the tool by running the following command from the etcd source directory.

```
  $ go install -v ./tools/etcd-dump-metrics
```

The installation will place executables in the $GOPATH/bin. If $GOPATH environment variable is not set, the tool will be
installed into the $HOME/go/bin. You can also find out the installed location by running the following command from the
etcd source directory. Make sure that $PATH is set accordingly in your environment.

```
  $ go list -f "{{.Target}}" ./tools/etcd-dump-metrics
```

Alternatively, instead of installing the tool, you can use it by simply running the following command from the etcd source
directory.

```
  $ go run ./tools/etcd-dump-metrics
```

## Usage

The following command should output the usage per the latest development.

```
  $ etcd-dump-metrics --help
```

An example of usage detail is provided below.

### For the latest main branch
```
  $ etcd-dump-metrics
```

### For the provided endpoint
```
  $ goreman start
  $ etcd-dump-metrics --addr http://localhost:2379/metrics
```

### Download specific version to temporary directory to fetch metrics
```
  $ etcd-dump-metrics --debug --download-ver v3.5.3
  $ etcd-dump-metrics --download-ver v3.5.3
```
