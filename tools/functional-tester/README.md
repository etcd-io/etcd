# etcd functional test suite

etcd functional test suite tests the functionality of an etcd cluster with a focus on failure resistance under high pressure. It sets up an etcd cluster and inject failures into the cluster by killing the process or isolate the network of the process. It expects the etcd cluster to recover within a short amount of time after fixing the fault.

etcd functional test suite has two components: etcd-agent and etcd-tester. etcd-agent runs on every test machine, and etcd-tester is a single controller of the test. tester controls agents: start etcd process, stop, terminate, inject failures, and so on.

### Run locally

```bash
PASSES=functional ./test
```

### Run with Docker

To run locally, first build tester image:

```bash
pushd ../..
make build-docker-functional-tester
popd
```

And run [example scripts](./scripts).

```bash
# run 3 agents for 3-node local etcd cluster
./scripts/docker-local-agent.sh 1
./scripts/docker-local-agent.sh 2
./scripts/docker-local-agent.sh 3

# to run only 1 tester round
./scripts/docker-local-tester.sh
```
