# Discovery Protocol

Starting a new etcd cluster can be painful since each machine needs to know of at least one live machine in the cluster. If you are trying to bring up a new cluster all at once, say using an AWS cloud formation, you also need to coordinate who will be the initial cluster leader. The discovery protocol uses an existing running etcd cluster to start a second etcd cluster.

To use this feature you add the command line flag `-discovery` to your etcd args. In this example we will use `http://example.com/v2/keys/_etcd/registry` as the URL prefix.

## The Protocol

By convention the etcd discovery protocol uses the key prefix `_etcd/registry`. A full URL to the keyspace will be `http://example.com/v2/keys/_etcd/registry`.

### Creating a New Cluster

Generate a unique token that will identify the new cluster. This will be used as a key prefix in the following steps. An easy way to do this is to use uuidgen:

```
UUID=$(uuidgen)
```

### Bringing up Machines

Now that you have your cluster ID you can start bringing up machines. Every machine will follow this protocol internally in etcd if given a `-discovery`.

### Registering your Machine

The first thing etcd must do is register your machine. This is done by using the machine name (from the `-name` arg) and posting it with a long TTL to the given key.

```
curl -X PUT "http://example.com/v2/keys/_etcd/registry/${UUID}/${etcd_machine_name}?ttl=604800" -d value=${peer_addr}
```

### Discovering Peers

Now that this etcd machine is registered it must discover its peers.

But, the tricky bit of starting a new cluster is that one machine needs to assume the initial role of leader and will have no peers. To figure out if another machine has already started the cluster etcd needs to create the `_state` key and set its value to "started":

```
curl -X PUT "http://example.com/v2/keys/_etcd/registry/${UUID}/_state?prevExist=false" -d value=started
```

If this returns a `200 OK` response then this machine is the initial leader and should start with no peers configured. If, however, this returns a `412 Precondition Failed` then you need to find all of the registered peers:

```
curl -X GET "http://example.com/v2/keys/_etcd/registry/${UUID}?recursive=true"
```

```
{
    "action": "get",
    "node": {
        "createdIndex": 11,
        "dir": true,
        "key": "/_etcd/registry/9D4258A5-A1D3-4074-8837-31C1E091131D",
        "modifiedIndex": 11,
        "nodes": [
            {
                "createdIndex": 16,
                "expiration": "2014-02-03T13:19:57.631253589-08:00",
                "key": "/_etcd/registry/9D4258A5-A1D3-4074-8837-31C1E091131D/peer1",
                "modifiedIndex": 16,
                "ttl": 604765,
                "value": "127.0.0.1:7001"
            },
            {
                "createdIndex": 17,
                "expiration": "2014-02-03T13:19:57.631253589-08:00",
                "key": "/_etcd/registry/9D4258A5-A1D3-4074-8837-31C1E091131D/peer2",
                "modifiedIndex": 17,
                "ttl": 604765,
                "value": "127.0.0.1:7002"
            }
        ]
    }
}
```

Using this information you can connect to the rest of the peers in the cluster.

### Heartbeating

At this point etcd will start heart beating to your registration URL. The
protocol uses a heartbeat so permanently deleted nodes get slowly removed from
the discovery information cluster.

The heartbeat interval is about once per day and the TTL is one week. This
should give a sufficiently wide window to protect against a discovery service
taking a temporary outage yet provide adequate cleanup.
