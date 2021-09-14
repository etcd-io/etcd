### etcd/tools/nodes-data-compare

etcd-node-data-compare compare data between etcd nodes.

```

Usage:
  etcd-nodes-data-compare [command]

Available Commands:
  compare     compare data between etcd nodes.
  help        Help about any command

Flags:
      --cacert string       verify certificates of HTTPS-enabled servers using this CA bundle
      --cert string         identify HTTPS client using this SSL certificate file
      --endpoints strings   gRPC endpoints (default [127.0.0.1:2379])
  -h, --help                help for etcd-nodes-data-compare
      --key string          identify HTTPS client using this SSL key file
      --maxGapRev int       Maximum current rev gap between nodes (1000 default) (default 1000)
      --port string         metrics http port
      --user string         provide username[:password] and prompt if password is not supplied.

Use "etcd-nodes-data-compare [command] --help" for more information about a command.


```

#### nodes-data-compare compare

```
$ nodes-data-compare compare --endpoints=$endpoints --cacert=$cacert --cert=$clientcert --key=$clientkey --port=$port

{"level":"info","msg":"compare revision range: ","maxRev":468889625,"minRevNew":468890009}
{"level":"info","msg":"compare success"}
{"level":"info","msg":"compare revision range: ","maxRev":468890009,"minRevNew":468890355}
{"level":"info","msg":"compare success"}
{"level":"info","msg":"compare revision range: ","maxRev":468890355,"minRevNew":468890709}
{"level":"info","msg":"compare success"}

```
