
## Why grpc-gateway

etcd v3 uses [gRPC][grpc] for its messaging protocol. The etcd project includes a gRPC-based [Go client][go-client] and a command line utility, [etcdctl][etcdctl], for communicating with an etcd cluster through gRPC. For languages with no gRPC support, etcd provides a JSON [grpc-gateway][grpc-gateway]. This gateway serves a RESTful proxy that translates HTTP/JSON requests into gRPC messages.


## Using grpc-gateway

The gateway accepts a [JSON mapping][json-mapping] for etcd's [protocol buffer][api-ref] message definitions. Note that `key` and `value` fields are defined as byte arrays and therefore must be base64 encoded in JSON. The following examples use `curl`, but any HTTP/JSON client should work all the same.

### Put and get keys

Use the `/v3beta/kv/range` and `/v3beta/kv/put` services to read and write keys:

```bash
<<COMMENT
https://www.base64encode.org/
foo is 'Zm9v' in Base64
bar is 'YmFy'
COMMENT

curl -L http://localhost:2379/v3beta/kv/put \
	-X POST -d '{"key": "Zm9v", "value": "YmFy"}'
# {"header":{"cluster_id":"12585971608760269493","member_id":"13847567121247652255","revision":"2","raft_term":"3"}}

curl -L http://localhost:2379/v3beta/kv/range \
	-X POST -d '{"key": "Zm9v"}'
# {"header":{"cluster_id":"12585971608760269493","member_id":"13847567121247652255","revision":"2","raft_term":"3"},"kvs":[{"key":"Zm9v","create_revision":"2","mod_revision":"2","version":"1","value":"YmFy"}],"count":"1"}

# get all keys prefixed with "foo"
curl -L http://localhost:2379/v3beta/kv/range \
	-X POST -d '{"key": "Zm9v", "range_end": "Zm9w"}'
# {"header":{"cluster_id":"12585971608760269493","member_id":"13847567121247652255","revision":"2","raft_term":"3"},"kvs":[{"key":"Zm9v","create_revision":"2","mod_revision":"2","version":"1","value":"YmFy"}],"count":"1"}
```

### Watch keys

Use the `/v3beta/watch` service to watch keys:

```bash
curl http://localhost:2379/v3beta/watch \
        -X POST -d '{"create_request": {"key":"Zm9v"} }' &
# {"result":{"header":{"cluster_id":"12585971608760269493","member_id":"13847567121247652255","revision":"1","raft_term":"2"},"created":true}}

curl -L http://localhost:2379/v3beta/kv/put \
	-X POST -d '{"key": "Zm9v", "value": "YmFy"}' >/dev/null 2>&1
# {"result":{"header":{"cluster_id":"12585971608760269493","member_id":"13847567121247652255","revision":"2","raft_term":"2"},"events":[{"kv":{"key":"Zm9v","create_revision":"2","mod_revision":"2","version":"1","value":"YmFy"}}]}}
```

### Transactions

Issue a transaction with `/v3beta/kv/txn`:

```bash
curl -L http://localhost:2379/v3beta/kv/txn \
	-X POST \
	-d '{"compare":[{"target":"CREATE","key":"Zm9v","createRevision":"2"}],"success":[{"requestPut":{"key":"Zm9v","value":"YmFy"}}]}'
# {"header":{"cluster_id":"12585971608760269493","member_id":"13847567121247652255","revision":"3","raft_term":"2"},"succeeded":true,"responses":[{"response_put":{"header":{"revision":"3"}}}]}
```

### Authentication

Set up authentication with the `/v3beta/auth` service:

```bash
# create root user
curl -L http://localhost:2379/v3beta/auth/user/add \
	-X POST -d '{"name": "root", "password": "pass"}'
# {"header":{"cluster_id":"14841639068965178418","member_id":"10276657743932975437","revision":"1","raft_term":"2"}}

# create root role
curl -L http://localhost:2379/v3beta/auth/role/add \
	-X POST -d '{"name": "root"}'
# {"header":{"cluster_id":"14841639068965178418","member_id":"10276657743932975437","revision":"1","raft_term":"2"}}

# grant root role
curl -L http://localhost:2379/v3beta/auth/user/grant \
	-X POST -d '{"user": "root", "role": "root"}'
# {"header":{"cluster_id":"14841639068965178418","member_id":"10276657743932975437","revision":"1","raft_term":"2"}}

# enable auth
curl -L http://localhost:2379/v3beta/auth/enable -X POST -d '{}'
# {"header":{"cluster_id":"14841639068965178418","member_id":"10276657743932975437","revision":"1","raft_term":"2"}}
```

Authenticate with etcd for an authentication token using `/v3beta/auth/authenticate`:

```bash
# get the auth token for the root user
curl -L http://localhost:2379/v3beta/auth/authenticate \
	-X POST -d '{"name": "root", "password": "pass"}'
# {"header":{"cluster_id":"14841639068965178418","member_id":"10276657743932975437","revision":"1","raft_term":"2"},"token":"sssvIpwfnLAcWAQH.9"}
```

Set the `Authorization` header to the authentication token to fetch a key using authentication credentials:

```bash
curl -L http://localhost:2379/v3beta/kv/put \
	-H 'Authorization : sssvIpwfnLAcWAQH.9' \
	-X POST -d '{"key": "Zm9v", "value": "YmFy"}'
# {"header":{"cluster_id":"14841639068965178418","member_id":"10276657743932975437","revision":"2","raft_term":"2"}}
```

## Swagger

Generated [Swagger][swagger] API definitions can be found at [rpc.swagger.json][swagger-doc].

[api-ref]: ./api_reference_v3.md
[go-client]: https://github.com/coreos/etcd/tree/master/clientv3
[etcdctl]: https://github.com/coreos/etcd/tree/master/etcdctl
[grpc]: http://www.grpc.io/
[grpc-gateway]: https://github.com/grpc-ecosystem/grpc-gateway
[json-mapping]: https://developers.google.com/protocol-buffers/docs/proto3#json
[swagger]: http://swagger.io/
[swagger-doc]: apispec/swagger/rpc.swagger.json

