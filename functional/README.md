# etcd Functional Testing

`functional` verifies the correct behavior of etcd under various system and network malfunctions. It sets up an etcd cluster under high pressure loads and continuously injects failures into the cluster. Then it expects the etcd cluster to recover within a few seconds. This has been extremely helpful to find critical bugs.

See [functional.yaml](../functional.yaml) for an example configuration.

### Run locally

```bash
PASSES=functional ./test
```

### Run with Docker

```bash
pushd ../..
make build-docker-functional
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
