This directory enables integration of Antithesis with etcd. There are 4 containers running in this system: 3 that make up an etcd cluster (etcd0, etcd1, etcd2) and one that "[makes the system go](https://antithesis.com/docs/getting_started/basic_test_hookup/)" (client).

# Running tests with docker compose

## Quickstart

### 1. Build and Tag the Docker Image

Run this command from the `antithesis/test-template` directory:

```bash
make antithesis-build-client-docker-image
make antithesis-build-etcd-image
```

Both commands build etcd-server and etcd-client from the current branch. To build a different version of etcd you can use:

```bash
make antithesis-build-etcd-image REF=${GIT_REF} 
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
make antithesis-docker-compose-up
```

The command uses the etcd client and server images built from step 1.

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

### 4. Running the tests

```bash
make antithesis-run-container-traffic
make antithesis-run-container-validation
```

Alternatively, with the etcd cluster from step 3, to run the tests locally without rebuilding the client image:

```bash
make antithesis-run-local-traffic
make antithesis-run-local-validation
```

### 5. Prepare for next run

Unfortunatelly robustness tests don't support running on non empty database.
So for now you need to cleanup the storage before repeating the run or you will get "non empty database at start, required by model used for linearizability validation" error.

```bash
make antithesis-clean
```

## Troubleshooting

- **Image Pull Errors**: If Docker can’t pull `etcd-client:latest`, make sure you built it locally (see the “Build and Tag” step) or push it to a registry that Compose can access.

# Running Tests with Kubernetes (WIP)

## Prerequisites

Please make sure that you have the following tools installed on your local:

- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start#installation)

## Testing locally

### Setting up the cluster and deploying the images

#### 1. Ensure your access to a test kubernetes cluster

You can use `kind` to create a local cluster to deploy the etcd-server and test client.  Once you have `kind` installed, you can use the following command to create a local cluster:

```bash
kind create cluster
```

Alternatively, you can use any existing kubernetes cluster you have access to.

#### 2. Build and load the images

Please [build the client and server images](#1-build-and-tag-the-docker-image) first. Then load the images into the `kind` cluster:

If you use `kind`, the cluster will need to have access to the images using the following commands:

```bash
kind load docker-image etcd-client:latest
kind load docker-image etcd-server:latest
```

If you use something other than `kind`, please make sure the images are accessible to your cluster. This might involve pushing the images to a container registry that your cluster can pull from.

#### 3. Deploy the kubernetes manifests

```bash
kubectl apply -f ./config/manifests
```

#### 4. Run the driver(s)

First, locate your client pod, which should be named with prefix client when you execute `kubectl get pods`.

```bash
> kubectl get pods
NAME                      READY   STATUS    RESTARTS   AGE
client-6b9885f69b-dcndw   1/1     Running   0          10m
etcd-0                    1/1     Running   0          10m
etcd-1                    1/1     Running   0          10m
etcd-2                    1/1     Running   0          10m
```

With the pod name, you can execute the traffic and validation commands:

```bash
# Traffic generation
kubectl exec -it client-6b9885f69b-dcndw -- /opt/antithesis/test/v1/robustness/single_driver_traffic

# Collecting results and compiling the reports
kubectl exec -it client-6b9885f69b-dcndw -- /opt/antithesis/test/v1/robustness/finally_validation
```

##### Making changes and retests

If you make changes to the client or server, you will have to rebuild the images and load them into the cluster again. You can use the same commands as in step 2. Once that the images are loaded, you can delete the existing pods to force kubernetes to create new ones with the updated images:

### Tearing down the cluster

```bash
kind delete cluster --name kind
```
