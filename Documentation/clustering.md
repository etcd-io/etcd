## Clustering

### Example cluster of three machines

Let's explore the use of etcd clustering.
We use Raft as the underlying distributed protocol which provides consistency and persistence of the data across all of the etcd instances.

Let start by creating 3 new etcd instances.

We use `-peer-addr` to specify server port and `-addr` to specify client port and `-data-dir` to specify the directory to store the log and info of the machine in the cluster:

```sh
./etcd -peer-addr 127.0.0.1:7001 -addr 127.0.0.1:4001 -data-dir machines/machine1 -name machine1
```

**Note:** If you want to run etcd on an external IP address and still have access locally, you'll need to add `-bind-addr 0.0.0.0` so that it will listen on both external and localhost addresses.
A similar argument `-peer-bind-addr` is used to setup the listening address for the server port.

Let's join two more machines to this cluster using the `-peers` argument. A single connection to any peer will allow a new machine to join, but multiple can be specified for greater resiliency.

```sh
./etcd -peer-addr 127.0.0.1:7002 -addr 127.0.0.1:4002 -peers 127.0.0.1:7001,127.0.0.1:7003 -data-dir machines/machine2 -name machine2
./etcd -peer-addr 127.0.0.1:7003 -addr 127.0.0.1:4003 -peers 127.0.0.1:7001,127.0.0.1:7002 -data-dir machines/machine3 -name machine3
```

We can retrieve a list of machines in the cluster using the HTTP API:

```sh
curl -L http://127.0.0.1:4001/v2/machines
```

We should see there are three machines in the cluster

```
http://127.0.0.1:4001, http://127.0.0.1:4002, http://127.0.0.1:4003
```

The machine list is also available via the main key API:

```sh
curl -L http://127.0.0.1:4001/v2/keys/_etcd/machines
```

```json
{
    "action": "get",
    "node": {
        "createdIndex": 1,
        "dir": true,
        "key": "/_etcd/machines",
        "modifiedIndex": 1,
        "nodes": [
            {
                "createdIndex": 1,
                "key": "/_etcd/machines/machine1",
                "modifiedIndex": 1,
                "value": "raft=http://127.0.0.1:7001&etcd=http://127.0.0.1:4001"
            },
            {
                "createdIndex": 2,
                "key": "/_etcd/machines/machine2",
                "modifiedIndex": 2,
                "value": "raft=http://127.0.0.1:7002&etcd=http://127.0.0.1:4002"
            },
            {
                "createdIndex": 3,
                "key": "/_etcd/machines/machine3",
                "modifiedIndex": 3,
                "value": "raft=http://127.0.0.1:7003&etcd=http://127.0.0.1:4003"
            }
        ]
    }
}
```

We can also get the current leader in the cluster:

```
curl -L http://127.0.0.1:4001/v2/leader
```

The first server we set up should still be the leader unless it has died during these commands.

```
http://127.0.0.1:7001
```

Now we can do normal SET and GET operations on keys as we explored earlier.

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar
```

```json
{
    "action": "set",
    "node": {
        "createdIndex": 4,
        "key": "/foo",
        "modifiedIndex": 4,
        "value": "bar"
    }
}
```

### Rejoining to the Cluster

If one machine disconnects from the cluster, it could rejoin the cluster automatically when the communication is recovered.

If one machine is killed, it could rejoin the cluster when started with old name. If the peer address is changed, etcd will treat the new peer address as the refreshed one, which benefits instance migration, or virtual machine boot with different IP. The peer-address-changing functionality is only supported when the majority of the cluster is alive, because this behavior needs the consensus of the etcd cluster.

**Note:** For now, it is user responsibility to ensure that the machine doesn't join the cluster that has the member with the same name. Or unexpected error will happen. It would be improved sooner or later.

### Killing Nodes in the Cluster

Now if we kill the leader of the cluster, we can get the value from one of the other two machines:

```sh
curl -L http://127.0.0.1:4002/v2/keys/foo
```

We can also see that a new leader has been elected:

```
curl -L http://127.0.0.1:4002/v2/leader
```

```
http://127.0.0.1:7002
```

or

```
http://127.0.0.1:7003
```


### Testing Persistence

Next we'll kill all the machines to test persistence.
Type `CTRL-C` on each terminal and then rerun the same command you used to start each machine.

Your request for the `foo` key will return the correct value:

```sh
curl -L http://127.0.0.1:4002/v2/keys/foo
```

```json
{
    "action": "get",
    "node": {
        "createdIndex": 4,
        "key": "/foo",
        "modifiedIndex": 4,
        "value": "bar"
    }
}
```


### Using HTTPS between servers

In the previous example we showed how to use SSL client certs for client-to-server communication.
Etcd can also do internal server-to-server communication using SSL client certs.
To do this just change the `-*-file` flags to `-peer-*-file`.

If you are using SSL for server-to-server communication, you must use it on all instances of etcd.

### Bootstrapping a new cluster by name

An etcd server is uniquely defined by the peer addresses it listens to. Suppose, however, that you wish to start over, while maintaining the data from the previous cluster -- that is, to pretend that this machine has never joined a cluster before.

You can use `--initial-cluster-name` to generate a new unique ID for each node, as a shared token that every node understands. Nodes also take this into account for bootstrapping the new cluster ID, so it also provides a way for a machine to listen on the same interfaces, disconnect from one cluster, and join a different cluster.
