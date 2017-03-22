## Versioning

### Service versioning

etcd uses [semantic versioning](http://semver.org)
New minor versions may add additional features to the API.

Get the running etcd cluster version with `etcdctl`:

```sh
ETCDCTL_API=3 etcdctl --endpoints=127.0.0.1:2379 endpoint status
```

### API versioning

The `v3` API responses should not change after the 3.0.0 release but new features will be added over time.


<!-- BEGIN ANALYTICS --> [![Analytics](http://ga-beacon.prod.coreos.systems/UA-42684979-9/github.com/coreos/etcd/Documentation/op-guide/versioning.md?pixel)]() <!-- END ANALYTICS -->
