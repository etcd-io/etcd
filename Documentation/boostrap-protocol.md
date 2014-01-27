# Bootstrap Protocol

Bootstrapping an etcd cluster can be painful since each node needs to know of another node in the cluster to get started. If you are trying to bring up a cluster all at once, say using a cloud formation, you also need to coordinate who will be the initial cluster leader. The bootstrapping protocol helps you by providing a way to bootstrap an etcd instance using another already running instance.

To enable use of this protocol you add the command line flag `-bootstrap-url` to your etcd args. In this example we will use `http://example.com/v2/keys/bootstrap/` as the URL prefix.

## The Protocol

By convention the etcd bootstrapping protocol uses the key prefix `bootstrap`. A full URL to the keyspace will be `http://example.com/v2/keys/bootstrap`.

## Creating a New Cluster

Generate a unique (secret) token that will identify the cluster and create a key called "_state". If you get a `201 Created` back then your key is unused and you can proceed with cluster creation. If the return value is `412 Precondition Failed` then you will need to create a new token.

```
UUID=$(uuidgen)
curl -X PUT "http://example.com/v2/keys/bootstrap/${UUID}/_state?prevExist=false?ttl=" -d value=init
```

## Bringing up Machines

Now that you have your cluster ID you can start bringing up machines. Every machine will follow this protocol internally in etcd if given a `-bootstrap-url`.

### Registering your Machine 

The first thing etcd must do is register your machine. This is done by using the machine name (from the `-name` arg) and posting it with a long TTL to the given key.

```
curl -X PUT "http://example.com/v2/keys/bootstrap/${UUID}/${etcd_machine_name}?ttl=604800" -d value=${peer_addr}
```

### Figuring out your Peers

Now that this etcd machine is registered it must figure out its peers.

But, the tricky bit of bootstrapping is that one machine needs to assume the initial role of leader and will have no peers. To figure out if another machine has taken on this etcd needs to update the `_state` key from "init" to "started":

```
curl -X PUT "http://example.com/v2/keys/bootstrap/${UUID}/_state?prevValue=init" -d value=started
```

If this returns a `200 OK` response then this machine is the initial leader and should start with no peers configured. If, however, this returns a `412 Precondition Failed` then you need to find all of the registered peers:

```
curl -X GET "http://example.com/v2/keys/bootstrap/${UUID}?recursive=true"
```

```
{
    "action": "get",
    "node": {
        "createdIndex": 11,
        "dir": true,
        "key": "/bootstrap/9D4258A5-A1D3-4074-8837-31C1E091131D",
        "modifiedIndex": 11,
        "nodes": [
            {
                "createdIndex": 16,
                "expiration": "2014-02-03T13:19:57.631253589-08:00",
                "key": "/bootstrap/9D4258A5-A1D3-4074-8837-31C1E091131D/peer1",
                "modifiedIndex": 16,
                "ttl": 604765,
                "value": "127.0.0.1:7001"
            },
            {
                "createdIndex": 17,
                "expiration": "2014-02-03T13:19:57.631253589-08:00",
                "key": "/bootstrap/9D4258A5-A1D3-4074-8837-31C1E091131D/peer2",
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

At this point you will want to heartbeat your registration URL every few hours. This will be done via a Go routine inside of etcd.