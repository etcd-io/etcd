# etcd on Kubernetes

## etcd w/out TLS

This is an example setting up etcd as a set of pods and services running on top of kubernetes. See [etcd.yml](etcd.yml). 

```
$ kubectl create -f etcd.yml 
services/etcd-client
pods/etcd0
services/etcd0
pods/etcd1
services/etcd1
pods/etcd2
services/etcd2
$ # now deploy a service that consumes etcd, such as vulcand
$ kubectl create -f vulcand.yml
```

## etcd with full client and peer TLS

In [etcd-tls.yml](etcd-tls.yml) we setup full peer and client based TLS using Kubernetes secrets objects. The script [gen-etcd-tls-secrets.sh](gen-etcd-tls-secrets.sh) is provided to help create the secrets objects required to do this ([etcd-ca](https://github.com/coreos/etcd-ca) is required for this script to work). [etcd-tls-secrets.yml.dist](etcd-tls-secrets.yml.dist) is provided in case you would like to test this without generating the secrets. 


```
$ ./gen-etcd-tls-secrets.sh > etcd-tls-secrets.yml
$ kubectl create -f etcd-tls-secrets.yml
secrets/etcd-peer-ca
secrets/etcd0-peer-key
secrets/etcd1-peer-key
secrets/etcd2-peer-key
secrets/etcd-client-ca
secrets/etcd-client-key
secrets/etcd0-client-key
secrets/etcd1-client-key
secrets/etcd2-client-key
$ kubectl create -f etcd-tls.yml
pods/etcd0
services/etcd0
pods/etcd1
services/etcd1
pods/etcd2
services/etcd2
```

A secret for clients is created and named `etcd-client-key`. Test it out with etcdctl. 

```
$ kubectl create -f etcdctl.yml
$ kubectl exec etcdctl -- /etcdctl \
-peers https://etcd0:2379,https://etcd1:2379,https://etcd2:2379 \
-ca-file=/etc/etcd/client-ca/ca.crt \
-cert-file=/etc/etcd/client-key/my.crt \
-key-file=/etc/etcd/client-key/my.key \
set /foo bar
```


TODO:

- create a replication controller like service that knows how to add and remove nodes from the cluster correctly