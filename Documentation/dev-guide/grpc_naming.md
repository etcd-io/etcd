---
title: gRPC naming and discovery
weight: 3500
description: "go-grpc: for resolving gRPC endpoints with an etcd backend"
---

etcd provides a gRPC resolver to support an alternative name system that fetches endpoints from etcd for discovering gRPC services. The underlying mechanism is based on watching updates to keys prefixed with the service name.

## Using etcd discovery with go-grpc

The etcd client provides a gRPC resolver for resolving gRPC endpoints with an etcd backend. The resolver is initialized with an etcd client:

```go
import (
	"go.etcd.io/etcd/v3/clientv3"
	resolver "go.etcd.io/etcd/v3/clientv3/naming/resolver"

	"google.golang.org/grpc"
)

cli, cerr := clientv3.NewFromURL("http://localhost:2379")
etcdResolver, err := resolver.NewBuilder(clus.RandClient());
conn, gerr := grpc.Dial("etcd://foo/bar/my-service", grpc.WithResolvers(etcdResolver))
```

## Managing service endpoints

The etcd resolver treats all keys under the prefix of the resolution target following a "/" (e.g., "foo/bar/my-service/") 
with JSON-encoded (historically go-grpc `naming.Update`) values as potential service endpoints. 
Endpoints are added to the service by creating new keys and removed from the service by deleting keys.

### Adding an endpoint

New endpoints can be added to the service through `etcdctl`:

```sh
ETCDCTL_API=3 etcdctl put foo/bar/my-service/1.2.3.4 '{"Addr":"1.2.3.4","Metadata":"..."}'
```

The etcd client's `endpoints.Manager` method can also register new endpoints with a key matching the `Addr`:

```go

em := endpoints.NewManager(client, "foo/bar/my-service")
err := em.AddEndpoint(context.TODO(),"foo/bar/my-service/e1", endpoints.Endpoint{Addr:"1.2.3.4"});
```

### Deleting an endpoint

Hosts can be deleted from the service through `etcdctl`:

```sh
ETCDCTL_API=3 etcdctl del foo/bar/my-service/1.2.3.4
```

The etcd client's `endpoints.Manager` method also supports deleting endpoints:

```go
em := endpoints.NewManager(client, "foo/bar/my-service")
err := em.DeleteEndpoint(context.TODO(), "foo/bar/my-service/e1");
```

### Registering an endpoint with a lease

Registering an endpoint with a lease ensures that if the host can't maintain a keepalive heartbeat (e.g., its machine fails), it will be removed from the service:

```sh
lease=`ETCDCTL_API=3 etcdctl lease grant 5 | cut -f2 -d' '`
ETCDCTL_API=3 etcdctl put --lease=$lease my-service/1.2.3.4 '{"Addr":"1.2.3.4","Metadata":"..."}'
ETCDCTL_API=3 etcdctl lease keep-alive $lease
```
In the golang: 

```go
em := endpoints.NewManager(client, "foo/bar/my-service")
err := endpoints.AddEndpoint(context.TODO(), "foo/bar/my-service/e1", endpoints.Endpoint{Addr:"1.2.3.4"});
```

### Atomically updating endpoints

If it's desired to modify multiple endpoints in a single transaction, `endpoints.Manager` can be used directly:

```
em := endpoints.NewManager(c, "foo")

err := em.Update(context.TODO(), []*endpoints.UpdateWithOpts{
    endpoints.NewDeleteUpdateOpts("foo/bar/my-service/e1", endpoints.Endpoint{Addr: "1.2.3.4"}),
	endpoints.NewAddUpdateOpts("foo/bar/my-service/e1", endpoints.Endpoint{Addr: "1.2.3.14"})})
```