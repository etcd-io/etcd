## etcd Functional Testing

[`functional`](https://godoc.org/github.com/coreos/etcd/functional) verifies the correct behavior of etcd under various system and network malfunctions. It sets up an etcd cluster under high pressure loads and continuously injects failures into the cluster. Then it expects the etcd cluster to recover within a few seconds. This has been extremely helpful to find critical bugs.

See [`rpcpb.Case`](https://godoc.org/github.com/coreos/etcd/functional/rpcpb#Case) for all failure cases.

See [functional.yaml](https://github.com/etcd-io/etcd/blob/master/functional.yaml) for an example configuration.

### Run locally

```bash
PASSES=functional ./test
```

### Run with Docker

```bash
pushd ..
make build-docker-functional
popd
```

And run [example scripts](./scripts).

```bash
# run 3 agents for 3-node local etcd cluster
./functional/scripts/docker-local-agent.sh 1
./functional/scripts/docker-local-agent.sh 2
./functional/scripts/docker-local-agent.sh 3

# to run only 1 tester round
./functional/scripts/docker-local-tester.sh
```

## etcd Proxy

Proxy layer that simulates various network conditions.

Test locally

```bash
$ ./build
$ ./bin/etcd

$ make build-functional

$ ./bin/etcd-proxy --help
$ ./bin/etcd-proxy --from localhost:23790 --to localhost:2379 --http-port 2378 --verbose

$ ETCDCTL_API=3 ./bin/etcdctl --endpoints localhost:2379 put foo bar
$ ETCDCTL_API=3 ./bin/etcdctl --endpoints localhost:23790 put foo bar
```

Proxy overhead per request is under 500μs

```bash
$ go build -v -o ./bin/benchmark ./tools/benchmark

$ ./bin/benchmark \
  --endpoints localhost:2379 \
  --conns 5 \
  --clients 15 \
  put \
  --key-size 48 \
  --val-size 50000 \
  --total 10000

<<COMMENT
Summary:
  Total:	8.4611 secs.
  Slowest:	0.1324 secs.
  Fastest:	0.0011 secs.
  Average:	0.0121 secs.
  Stddev:	0.0125 secs.
  Requests/sec:	1181.8758

Response time histogram:
  0.0011 [1]	|
  0.0142 [7899]	|∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎
  0.0273 [1339]	|∎∎∎∎∎∎
  0.0405 [543]	|∎∎
  0.0536 [67]	|
  0.0667 [49]	|
  0.0798 [9]	|
  0.0930 [15]	|
  0.1061 [42]	|
  0.1192 [21]	|
  0.1324 [15]	|

Latency distribution:
  10% in 0.0049 secs.
  25% in 0.0064 secs.
  50% in 0.0085 secs.
  75% in 0.0126 secs.
  90% in 0.0243 secs.
  95% in 0.0307 secs.
  99% in 0.0686 secs.
  99.9% in 0.1294 secs.
COMMENT

$ ./bin/benchmark \
  --endpoints localhost:23790 \
  --conns 5 \
  --clients 15 \
  put \
  --key-size 48 \
  --val-size 50000 \
  --total 10000

<<COMMENT
Summary:
  Total:	9.1128 secs.
  Slowest:	0.1363 secs.
  Fastest:	0.0015 secs.
  Average:	0.0131 secs.
  Stddev:	0.0113 secs.
  Requests/sec:	1097.3613

Response time histogram:
  0.0015 [1]	|
  0.0150 [7407]	|∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎
  0.0285 [2017]	|∎∎∎∎∎∎∎∎∎∎
  0.0419 [440]	|∎∎
  0.0554 [30]	|
  0.0689 [13]	|
  0.0824 [12]	|
  0.0959 [48]	|
  0.1093 [2]	|
  0.1228 [16]	|
  0.1363 [14]	|

Latency distribution:
  10% in 0.0054 secs.
  25% in 0.0071 secs.
  50% in 0.0100 secs.
  75% in 0.0153 secs.
  90% in 0.0241 secs.
  95% in 0.0297 secs.
  99% in 0.0584 secs.
  99.9% in 0.1312 secs.
COMMENT
```

Delay client transmit

```bash
$ curl -L http://localhost:2378/delay-tx -X PUT \
  -d "latency=5s&random-variable=100ms"
# added send latency 5s±100ms (current latency 4.92143955s)

$ curl -L http://localhost:2378/delay-tx
# current send latency 4.92143955s

$ ETCDCTL_API=3 ./bin/etcdctl \
  --endpoints localhost:23790 \
  --command-timeout=3s \
  put foo bar
# Error: context deadline exceeded

$ curl -L http://localhost:2378/delay-tx -X DELETE
# removed latency 4.92143955s

$ curl -L http://localhost:2378/delay-tx
# current send latency 0s

$ ETCDCTL_API=3 ./bin/etcdctl \
  --endpoints localhost:23790 \
  --command-timeout=3s \
  put foo bar
# OK
```

Pause client transmit

```bash
$ curl -L http://localhost:2378/pause-tx -X PUT
# paused forwarding [tcp://localhost:23790 -> tcp://localhost:2379]

$ ETCDCTL_API=3 ./bin/etcdctl \
  --endpoints localhost:23790 \
  put foo bar
# Error: context deadline exceeded

$ curl -L http://localhost:2378/pause-tx -X DELETE
# unpaused forwarding [tcp://localhost:23790 -> tcp://localhost:2379]
```

Drop client packets

```bash
$ curl -L http://localhost:2378/blackhole-tx -X PUT
# blackholed; dropping packets [tcp://localhost:23790 -> tcp://localhost:2379]

$ ETCDCTL_API=3 ./bin/etcdctl --endpoints localhost:23790 put foo bar
# Error: context deadline exceeded

$ curl -L http://localhost:2378/blackhole-tx -X DELETE
# unblackholed; restart forwarding [tcp://localhost:23790 -> tcp://localhost:2379]
```

Trigger leader election

```bash
$ ./build
$ make build-functional

$ rm -rf /tmp/etcd-proxy-data.s*
$ goreman -f ./functional/Procfile-proxy start

$ ETCDCTL_API=3 ./bin/etcdctl \
  --endpoints localhost:13790,localhost:23790,localhost:33790 \
  member list

# isolate s1 when s1 is the current leader
$ curl -L http://localhost:1381/blackhole-tx -X PUT
$ curl -L http://localhost:1381/blackhole-rx -X PUT
# s1 becomes follower after election timeout
```
