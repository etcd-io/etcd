# Kubernetes managing an etcd2 cluster

The goal of this project is to enable Kubernetes to manage an etcd2 cluster and to keep it healthy

It accomplishes this by using persistent storage that will always be available to a given pod even as it is recreated

The example files provided here assume a cluster of size 3, but are generalizable to a any fixed sized cluster

## Architecture

In order for an etcd2 cluster to work without any discovery we need to have static IP addresses that are well known for each server instance.  We accomplish that with creating individual kubernetes' services for each replication controller/pod that will be part of our cluster.  Using kubernetes dns or environment variables lets each pod's etcd server know how to connect to the servers in the cluster.  Each of the N replication controller/pod instances will define a label that specifies app=etcd-N and each of the N services will select on app=etcd-N

In order to provide a consistent endpoint we also provide a global service that selects all of the replication controller/pod instances that make up our cluster and this is what clients should use.

In order to provide a consistent environment, we create N persistent volumes for etcd2 to store it's data with N persistent volumes claims.  Therefore, even if a pod is removed, on recreation, it will still have its data directory available and will come up cleanly.  This will only work if the environment can provide that volume on the different nodes, if for instance the persistent volume is node local storage and the node goes down, it wont help.

## Example instructions

These examples assume a single node kubernetes installation and use local storage to define the persistent volumes.  In a production environment with multiple nodes would need a network accessible storage, such as EBS on AWS or a Persistent Disk on GCE.

### Setup Kubernetes

1. launch etcd2 for kubernetes

  ```bash
docker run --net=host -d gcr.io/google_containers/etcd:2.0.12 /usr/local/bin/etcd --addr=127.0.0.1:4001 --bind-addr=0.0.0.0:4001 --data-dir=/var/etcd/data
  ```

2. launch kubernetes master

   ```bash
docker run \
    --volume=/:/rootfs:ro \
    --volume=/sys:/sys:ro \
    --volume=/dev:/dev \
    --volume=/var/lib/docker/:/var/lib/docker:ro \
    --volume=/var/lib/kubelet/:/var/lib/kubelet:rw \
    --volume=/var/run:/var/run:rw \
    --net=host \
    --pid=host \
    --privileged=true \
    -d \
    gcr.io/google_containers/hyperkube:v1.0.1 \
    /hyperkube kubelet --containerized --hostname-override="127.0.0.1" --address="0.0.0.0" --api-servers=http://localhost:8080 --config=/etc/kubernetes/manifests
```

3. launch service proxy

  ```bash
docker run -d --net=host --privileged gcr.io/google_containers/hyperkube:v1.0.1 /hyperkube proxy --master=http://127.0.0.1:8080 --v=2
```

4. configure kubectl for this instance

  ```bash
kubectl config set-cluster test-doc --server=http://localhost:8080
kubectl config set-context test-doc --cluster=test-doc
kubectl config use-context test-doc
```

### Creating the Kubernetes managed etcd2 cluster

1. Create the persistent volumes

 Remember, this uses local storage (hostPath) and will not work in a multinode cluster

 ```bash
  mkdir /tmp/v{1,2,3}
```

 ```bash
 $ kubectl create -f pv

 persistentvolumes/pv1
 persistentvolumes/pv2
 persistentvolumes/pv3
 ```

 ```bash
 $ kubectl get pv

 NAME      LABELS       CAPACITY     ACCESSMODES   STATUS      CLAIM     REASON
 pv1       type=local   1073741824   RWO           Available             
 pv2       type=local   1073741824   RWO           Available             
 pv3       type=local   1073741824   RWO           Available
 ```

2. Create the persistent volume claims

 Can't load a directory full of these claims as they are not setup correctly if done that way or if loaded very quickly within a for loop

 ```bash
 $ kubectl create -f claims/c1.yml

 persistentvolumeclaims/myclaim-1
 ```

 ```bash
 $ kubectl create -f claims/c2.yml

 persistentvolumeclaims/myclaim-2
 ```

 ```bash
 $ kubectl create -f claims/c3.yml

 persistentvolumeclaims/myclaim-3
 ```

 ```bash
 $ kubectl get pvc

 NAME        LABELS    STATUS    VOLUME
 myclaim-1   map[]     Bound     pv3
 myclaim-2   map[]     Bound     pv2
 myclaim-3   map[]     Bound     pv1
 ```

3. Create services

 This creates the kubernetes services that will be used by each individual pod/replicaiton controller as well as the global service that clients will use to hit the cluster

 ```bash
 $ kubectl create -f services/

 services/etcd-client
 services/etcd-1
 services/etcd-2
 services/etcd-3
 ```
 ```bash
 $ kubectl get services

 NAME          LABELS                                    SELECTOR       IP(S)        PORT(S)
 etcd-1        <none>                                    app=etcd-1     10.0.0.192   7001/TCP
                                                                                    4001/TCP
 etcd-2        <none>                                    app=etcd-2     10.0.0.237   7001/TCP
                                                                                    4001/TCP
 etcd-3        <none>                                    app=etcd-3     10.0.0.54    7001/TCP
                                                                                    4001/TCP
 etcd-client   <none>                                    service=etcd   10.0.0.113   7001/TCP
                                                                                    4001/TCP
 kubernetes    component=apiserver,provider=kubernetes   <none>         10.0.0.1     443/TCP
 ```

4. Create replication controllers / pods

 ```bash
 kubectl create -f rc/

 replicationcontrollers/etcd-1
 replicationcontrollers/etcd-2
 replicationcontrollers/etcd-3
 ```

 eventually each pod created by the replication controllers will be ready

 ```bash
 $ kubectl get pods

 NAME                   READY     STATUS    RESTARTS   AGE
 etcd-1-2966t           1/1       Running   0          10m
 etcd-2-mf4l9           1/1       Running   0          10m
 etcd-3-knja0           1/1       Running   0          10m
 k8s-master-127.0.0.1   3/3       Running   3          2h
 ```

### Testing the cluster

1. Monitoring healthy

 ```bash
 $ etcdctl --peers http://10.0.0.113:4001 cluster-health

 member 6314a7c82afdad2e is healthy: got healthy result from http://10.0.0.192:4001
 member 8520e7a649bd3f6e is healthy: got healthy result from http://10.0.0.54:4001
 member aa4c88abc01033b5 is healthy: got healthy result from http://10.0.0.237:4001
 cluster is healthy
 ```

2. Delete a pod

 ```bash
 $ kubectl delete pod etcd-3-knja0 && etcdctl --peers http://10.0.0.113:4001 cluster-health

 pods/etcd-3-knja0

 member 6314a7c82afdad2e is healthy: got healthy result from http://10.0.0.192:4001
 failed to check the health of member 8520e7a649bd3f6e on http://10.0.0.54:4001: Get http://10.0.0.54:4001/health: read tcp 10.7.3.187:60956->10.0.0.54:4001: read: connection reset by peer
 member 8520e7a649bd3f6e is unreachable: [http://10.0.0.54:4001] are all unreachable
 member aa4c88abc01033b5 is healthy: got healthy result from http://10.0.0.237:4001
 cluster is healthy
 ```

 wait a moment for the rc to recreate the pod

 ```bash
 $ kubectl get pods

 NAME                   READY     STATUS    RESTARTS   AGE
 etcd-1-2966t           1/1       Running   1          19m
 etcd-2-mf4l9           1/1       Running   1          19m
 etcd-3-hlj3u           1/1       Running   0          6m
 k8s-master-127.0.0.1   3/3       Running   3          2h
 ```

 ```bash
 $ etcdctl --peers http://10.0.0.113:4001 cluster-health

 member 6314a7c82afdad2e is healthy: got healthy result from http://10.0.0.192:4001
 member 8520e7a649bd3f6e is healthy: got healthy result from http://10.0.0.54:4001
 member aa4c88abc01033b5 is healthy: got healthy result from http://10.0.0.237:4001
 cluster is healthy
 ```
