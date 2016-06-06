# Managing an etcd cluster with Kubernetes

This project includes guidance and sample files to use Kubernetes for managing and monitoring an etcd cluster. Persistent storage backs each pod as it is created, destroyed, and recreated, in order to provide the reliable state required by etcd. This example constructs an etcd cluster of three nodes, but can be extrapolated to any cluster size.

## Architecture

In order for an etcd cluster to work without any discovery we need to have static IP addresses that are well known for each server instance.  We accomplish that with creating individual Kubernetes services for each replication controller/pod in our cluster.  Using Kubernetes DNS or environment variables lets each pod's etcd know how to connect to the servers in the cluster.  Each of the N replication controller/pod instances will define a label that specifies `app=etcd-N` and each of the N services will select on `app=etcd-N`.

In order to provide a consistent endpoint we also provide a global service that selects all of the replication controller/pod instances that make up our cluster and this is what clients should use.

In order to provide a consistent environment, we create N persistent volumes for etcd to store it's data with N persistent volumes claims.  Therefore, even if a pod is removed, on recreation, it will still have its data directory available and will come up cleanly.  As replication controllers can recreate pods on different nodes if the initial node disappears, the persistent volumes must be located on network accessible storage in a production environment otherwise the restarted etcd pod will not have access to its data.

## Example instructions

This example assume a single node Kubernetes installation and uses local storage to define the persistent volumes.  A production environment, with multiple nodes, needs the the persistent volumes  be located on network accessible storage, such as EBS on AWS, or a Persistent Disk on GCE.

### Setup Kubernetes

1. launch etcd for Kubernetes

  ```bash
$ docker run --net=host -d --net=host -d gcr.io/google_containers/etcd:2.2.1 /usr/local/bin/etcd --addr=127.0.0.1:4001 --bind-addr=0.0.0.0:4001 --data-dir=/var/etcd/data
  ```

2. launch Kubernetes master

   ```bash
$ docker run \
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
    /hyperkube kubelet --containerized --hostname-override="127.0.0.1" --address="0.0.0.0" --api-servers=http://localhost:8080 --config=/etc/Kubernetes/manifests
```

3. launch service proxy

  ```bash
$ docker run -d --net=host --privileged gcr.io/google_containers/hyperkube:v1.0.1 /hyperkube proxy --master=http://127.0.0.1:8080 --v=2
```

4. configure kubectl for this instance

  ```bash
kubectl config set-cluster test-doc --server=http://localhost:8080
kubectl config set-context test-doc --cluster=test-doc
kubectl config use-context test-doc
```

### Creating the Kubernetes managed etcd cluster

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

 This creates the Kubernetes services that will be used by each individual pod/replicaiton controller as well as the global service that clients will use to hit the cluster

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
 etcd-1        <none>                                    app=etcd-1     10.0.0.241   2380/TCP
                                                                                     2379/TCP
 etcd-2        <none>                                    app=etcd-2     10.0.0.17    2380/TCP
                                                                                     2379/TCP
 etcd-3        <none>                                    app=etcd-3     10.0.0.149   2380/TCP
                                                                                     2379/TCP
 etcd-client   <none>                                    service=etcd   10.0.0.122   2380/TCP
                                                                                     2379/TCP
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
 etcd-1-r4obe           1/1       Running   0          2m
 etcd-2-x6kel           1/1       Running   0          5m
 etcd-3-hku9t           1/1       Running   0          5m
 k8s-master-127.0.0.1   3/3       Running   6          14m
 ```

### Testing the cluster

1. Monitoring a healthy cluster

 using etcd-client service ip

 ```bash
 $ etcdctl --peers http://10.0.0.122:2379 cluster-health

 member 57f9d3e3f9ee006 is healthy: got healthy result from http://10.0.0.241:2379
 member 54f0e7837cf89bc8 is healthy: got healthy result from http://10.0.0.17:2379
 member 8efc649d6465b98f is healthy: got healthy result from http://10.0.0.149:2379
 cluster is healthy
 ```

2. Delete a pod

 ```bash
 $ kubectl delete pod etcd-3-hku9t && etcdctl --peers http://10.0.0.122:2379 cluster-health

 pods/etcd-3-hku9t

 member 57f9d3e3f9ee006 is healthy: got healthy result from http://10.0.0.241:2379
 member 54f0e7837cf89bc8 is healthy: got healthy result from http://10.0.0.17:2379
 failed to check the health of member 8efc649d6465b98f on http://10.0.0.149:2379: Get http://10.0.0.149:2379/health: read tcp 192.168.0.249:43194->10.0.0.149:2379: read: connection reset by peer
 member 8efc649d6465b98f is unreachable: [http://10.0.0.149:2379] are all unreachable
 cluster is healthy
 ```

 wait a moment for the replication controller to recreate the pod

 ```bash
 $ kubectl get pods

 NAME                   READY     STATUS    RESTARTS   AGE
 etcd-1-r4obe           1/1       Running   0          5m
 etcd-2-x6kel           1/1       Running   0          7m
 etcd-3-exnm7           0/1       Running   0          29s
 k8s-master-127.0.0.1   3/3       Running   6          16m
 ```

 all members of the cluster are healthy

 ```bash
 $ etcdctl --peers http://10.0.0.122:2379 cluster-health

member 57f9d3e3f9ee006 is healthy: got healthy result from http://10.0.0.241:2379
member 54f0e7837cf89bc8 is healthy: got healthy result from http://10.0.0.17:2379
member 8efc649d6465b98f is healthy: got healthy result from http://10.0.0.149:2379
cluster is healthy
 ```
