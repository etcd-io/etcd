This directory enables integration of Antithesis with etcd. There are 4 containers running in this system: 3 that make up an etcd cluster (etcd0, etcd1, etcd2) and one that "[makes the system go](https://antithesis.com/docs/getting_started/basic_test_hookup/)" (client).

## Quickstart

### 1. Build and Tag the Docker Image

Run this command from the `antithesis/test-template` directory:

```bash
docker build . -f Dockerfile.client -t etcd-client:latest
```

### 2. (Optional) Check the Image Locally

You can verify your new image is built:

```bash
docker images | grep etcd-client
```

It should show something like:

```
etcd-client        latest    <IMAGE_ID>    <DATE>
```

### 3. Use in Docker Compose

Run the following command from the root directory for Antithesis tests (`tests/antithesis`):

```bash
docker-compose up
```

The client will continuously check the health of the etcd nodes and print logs similar to:

```
[+] Running 4/4
 ✔ Container etcd0   Created                                                                                                                                                 0.0s 
 ✔ Container etcd2   Created                                                                                                                                                 0.0s 
 ✔ Container etcd1   Created                                                                                                                                                 0.0s 
 ✔ Container client  Recreated                                                                                                                                               0.1s 
Attaching to client, etcd0, etcd1, etcd2
etcd2   | {"level":"info","ts":"2025-04-14T07:23:25.134294Z","caller":"flags/flag.go:113","msg":"recognized and used environment variable","variable-name":"ETCD_ADVERTISE_CLIENT_URLS","variable-value":"http://etcd2.etcd:2379"}
etcd2   | {"level":"info","ts":"2025-04-14T07:23:25.138501Z","caller":"flags/flag.go:113","msg":"recognized and used environment variable","variable-name":"ETCD_INITIAL_ADVERTISE_PEER_URLS","variable-value":"http://etcd2:2380"}
etcd2   | {"level":"info","ts":"2025-04-14T07:23:25.138646Z","caller":"flags/flag.go:113","msg":"recognized and used environment variable","variable-name":"ETCD_INITIAL_CLUSTER","variable-value":"etcd0=http://etcd0:2380,etcd1=http://etcd1:2380,etcd2=http://etcd2:2380"}
etcd0   | {"level":"info","ts":"2025-04-14T07:23:25.138434Z","caller":"flags/flag.go:113","msg":"recognized and used environment variable","variable-name":"ETCD_ADVERTISE_CLIENT_URLS","variable-value":"http://etcd0.etcd:2379"}
etcd0   | {"level":"info","ts":"2025-04-14T07:23:25.138582Z","caller":"flags/flag.go:113","msg":"recognized and used environment variable","variable-name":"ETCD_INITIAL_ADVERTISE_PEER_URLS","variable-value":"http://etcd0:2380"}
etcd0   | {"level":"info","ts":"2025-04-14T07:23:25.138592Z","caller":"flags/flag.go:113","msg":"recognized and used environment variable","variable-name":"ETCD_INITIAL_CLUSTER","variable-value":"etcd0=http://etcd0:2380,etcd1=http://etcd1:2380,etcd2=http://etcd2:2380"}

...
...
(skipping some repeated logs for brevity)
...
...

etcd2   | {"level":"info","ts":"2025-04-14T07:23:25.484698Z","caller":"etcdmain/main.go:50","msg":"successfully notified init daemon"}
etcd1   | {"level":"info","ts":"2025-04-14T07:23:25.484092Z","caller":"embed/serve.go:210","msg":"serving client traffic insecurely; this is strongly discouraged!","traffic":"grpc+http","address":"[::]:2379"}
etcd0   | {"level":"info","ts":"2025-04-14T07:23:25.484563Z","caller":"etcdmain/main.go:50","msg":"successfully notified init daemon"}
etcd2   | {"level":"info","ts":"2025-04-14T07:23:25.485101Z","caller":"v3rpc/health.go:61","msg":"grpc service status changed","service":"","status":"SERVING"}
etcd1   | {"level":"info","ts":"2025-04-14T07:23:25.484130Z","caller":"etcdmain/main.go:44","msg":"notifying init daemon"}
etcd2   | {"level":"info","ts":"2025-04-14T07:23:25.485782Z","caller":"embed/serve.go:210","msg":"serving client traffic insecurely; this is strongly discouraged!","traffic":"grpc+http","address":"[::]:2379"}
etcd1   | {"level":"info","ts":"2025-04-14T07:23:25.484198Z","caller":"etcdmain/main.go:50","msg":"successfully notified init daemon"}
client  | Client [entrypoint]: starting...
client  | Client [entrypoint]: checking cluster health...
client  | Client [entrypoint]: connection successful with etcd0
client  | Client [entrypoint]: connection successful with etcd1
client  | Client [entrypoint]: connection successful with etcd2
client  | Client [entrypoint]: cluster is healthy!
```

And it will stay running indefinitely.

## Troubleshooting

- **Image Pull Errors**: If Docker can’t pull `etcd-client:latest`, make sure you built it locally (see the “Build and Tag” step) or push it to a registry that Compose can access.
