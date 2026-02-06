# etcd-dump-db

`etcd-dump-db` inspects etcd db files.

## Installation

Install the tool by running the following command from the etcd source directory.

```
  $ go install -v ./tools/etcd-dump-db
```

The installation will place executables in the $GOPATH/bin. If $GOPATH environment variable is not set, the tool will be installed into the $HOME/go/bin. You can also find out the installed location by running the following command from the etcd source directory. Make sure that $PATH is set accordingly in your environment.

```
  $ go list -f "{{.Target}}" ./tools/etcd-dump-db
```

Alternatively, instead of installing the tool, you can use it by simply running the following command from the etcd source directory.

```
  $ go run ./tools/etcd-dump-db
```

## Usage

The following command should output the usage per the latest development.

```
  $ etcd-dump-db --help
```

An example of usage detail is provided below.

```
Usage:
  etcd-dump-db [command]

Available Commands:
  list-bucket    bucket lists all buckets.
  iterate-bucket iterate-bucket lists key-value pairs in reverse order.
  hash           hash computes the hash of db file.

Flags:
  -h, --help[=false]: help for etcd-dump-db

Use "etcd-dump-db [command] --help" for more information about a command.
```


#### list-bucket [data dir or db file path]

Lists all buckets.

```
$ etcd-dump-db list-bucket agent01/agent.etcd

alarm
auth
authRoles
authUsers
cluster
key
lease
members
members_removed
meta
```


#### hash [data dir or db file path]

Computes the hash of db file.

```
$ etcd-dump-db hash agent01/agent.etcd
db path: agent01/agent.etcd/member/snap/db
Hash: 3700260467


$ etcd-dump-db hash agent02/agent.etcd

db path: agent02/agent.etcd/member/snap/db
Hash: 3700260467


$ etcd-dump-db hash agent03/agent.etcd

db path: agent03/agent.etcd/member/snap/db
Hash: 3700260467
```


#### iterate-bucket [data dir or db file path]

Lists key-value pairs in reverse order.

```
$ etcd-dump-db iterate-bucket agent03/agent.etcd key --limit 3

key="\x00\x00\x00\x00\x005@x_\x00\x00\x00\x00\x00\x00\x00\tt", value="\n\x153640412599896088633_9"
key="\x00\x00\x00\x00\x005@x_\x00\x00\x00\x00\x00\x00\x00\bt", value="\n\x153640412599896088633_8"
key="\x00\x00\x00\x00\x005@x_\x00\x00\x00\x00\x00\x00\x00\at", value="\n\x153640412599896088633_7"
```

#### scan-keys [data dir or db file path]

Scans all the key-value pairs starting from a specific revision in the key space. It works even the db is corrupted.

```
$ ./etcd-dump-db scan-keys ~/tmp/etcd/778/db.db 16589739 2>/dev/null | grep "/registry/configmaps/istio-system/istio-namespace-controller-election"
pageID=1306, index=5/5, rev={Revision:{Main:16589739 Sub:0} tombstone:false}, value=[key "/registry/configmaps/istio-system/istio-namespace-controller-election" | val "k8s\x00\n\x0f\n\x02v1\x12\tConfigMap\x12\xeb\x03\n\xe8\x03\n#istio-namespace-controller-election\x12\x00\x1a\fistio-system\"\x00*$bb696087-260d-4167-bf06-17d3361f9b5f2\x008\x00B\b\b\x9e\xbe\xed\xb5\x06\x10\x00b\xe6\x01\n(control-plane.alpha.kubernetes.io/leader\x12\xb9\x01{\"holderIdentity\":\"istiod-d56968787-txq2d\",\"holderKey\":\"default\",\"leaseDurationSeconds\":30,\"acquireTime\":\"2024-08-13T13:26:54Z\",\"renewTime\":\"2024-08-27T06:16:13Z\",\"leaderTransitions\":0}\x8a\x01\x90\x01\n\x0fpilot-discovery\x12\x06Update\x1a\x02v1\"\b\b\xad\u07b5\xb6\x06\x10\x002\bFieldsV1:[\nY{\"f:metadata\":{\"f:annotations\":{\".\":{},\"f:control-plane.alpha.kubernetes.io/leader\":{}}}}B\x00\x1a\x00\"\x00" | created 9612546 | mod 16589739 | ver 157604]
pageID=4737, index=4/4, rev={Revision:{Main:16589786 Sub:0} tombstone:false}, value=[key "/registry/configmaps/istio-system/istio-namespace-controller-election" | val "k8s\x00\n\x0f\n\x02v1\x12\tConfigMap\x12\xeb\x03\n\xe8\x03\n#istio-namespace-controller-election\x12\x00\x1a\fistio-system\"\x00*$bb696087-260d-4167-bf06-17d3361f9b5f2\x008\x00B\b\b\x9e\xbe\xed\xb5\x06\x10\x00b\xe6\x01\n(control-plane.alpha.kubernetes.io/leader\x12\xb9\x01{\"holderIdentity\":\"istiod-d56968787-txq2d\",\"holderKey\":\"default\",\"leaseDurationSeconds\":30,\"acquireTime\":\"2024-08-13T13:26:54Z\",\"renewTime\":\"2024-08-27T06:16:21Z\",\"leaderTransitions\":0}\x8a\x01\x90\x01\n\x0fpilot-discovery\x12\x06Update\x1a\x02v1\"\b\b\xb5\u07b5\xb6\x06\x10\x002\bFieldsV1:[\nY{\"f:metadata\":{\"f:annotations\":{\".\":{},\"f:control-plane.alpha.kubernetes.io/leader\":{}}}}B\x00\x1a\x00\"\x00" | created 9612546 | mod 16589786 | ver 157605]
```