# etcd API

## Running a Single Machine Cluster

These examples will use a single member cluster to show you the basics of the etcd REST API.
Let's start etcd:

```sh
./bin/etcd
```

This will bring up etcd listening on the IANA assigned ports and listening on localhost.
The IANA assigned ports for etcd are 2379 for client communication and 2380 for server-to-server communication.

## Getting the etcd version

The etcd version of a specific instance can be obtained from the `/version` endpoint.

```sh
curl -L http://127.0.0.1:2379/version
```

## Key Space Operations

The primary API of etcd is a hierarchical key space.
The key space consists of directories and keys which are generically referred to as "nodes".

### Setting the value of a key

Let's set the first key-value pair in the datastore.
In this case the key is `/message` and the value is `Hello world`.

```sh
curl http://127.0.0.1:2379/v2/keys/message -XPUT -d value="Hello world"
```

```json
{
    "action": "set",
    "node": {
        "createdIndex": 2,
        "key": "/message",
        "modifiedIndex": 2,
        "value": "Hello world"
    }
}
```

The response object contains several attributes:

1. `action`: the action of the request that was just made.
The request attempted to modify `node.value` via a `PUT` HTTP request, thus the value of action is `set`.

2. `node.key`: the HTTP path to which the request was made.
We set `/message` to `Hello world`, so the key field is `/message`.
etcd uses a file-system-like structure to represent the key-value pairs, therefore all keys start with `/`.

3. `node.value`: the value of the key after resolving the request.
In this case, a successful request was made that attempted to change the node's value to `Hello world`.

4. `node.createdIndex`: an index is a unique, monotonically-incrementing integer created for each change to etcd.
This specific index reflects the point in the etcd state member at which a given key was created.
You may notice that in this example the index is `2` even though it is the first request you sent to the server.
This is because there are internal commands that also change the state behind the scenes, like adding and syncing servers.

5. `node.modifiedIndex`: like `node.createdIndex`, this attribute is also an etcd index.
Actions that cause the value to change include `set`, `delete`, `update`, `create`, `compareAndSwap` and `compareAndDelete`.
Since the `get` and `watch` commands do not change state in the store, they do not change the value of `node.modifiedIndex`.


### Response Headers

etcd includes a few HTTP headers in responses that provide global information about the etcd cluster that serviced a request:

```
X-Etcd-Index: 35
X-Raft-Index: 5398
X-Raft-Term: 1
```

- `X-Etcd-Index` is the current etcd index as explained above.
- `X-Raft-Index` is similar to the etcd index but is for the underlying raft protocol
- `X-Raft-Term` is an integer that will increase whenever an etcd master election happens in the cluster. If this number is increasing rapidly, you may need to tune the election timeout. See the [tuning][tuning] section for details.

[tuning]: #tuning


### Get the value of a key

We can get the value that we just set in `/message` by issuing a `GET` request:

```sh
curl http://127.0.0.1:2379/v2/keys/message
```

```json
{
    "action": "get",
    "node": {
        "createdIndex": 2,
        "key": "/message",
        "modifiedIndex": 2,
        "value": "Hello world"
    }
}
```


### Changing the value of a key

You can change the value of `/message` from `Hello world` to `Hello etcd` with another `PUT` request to the key:

```sh
curl http://127.0.0.1:2379/v2/keys/message -XPUT -d value="Hello etcd"
```

```json
{
    "action": "set",
    "node": {
        "createdIndex": 3,
        "key": "/message",
        "modifiedIndex": 3,
        "value": "Hello etcd"
    },
    "prevNode": {
    	"createdIndex": 2,
    	"key": "/message",
    	"value": "Hello world",
    	"modifiedIndex": 2
    }
}
```

Here we introduce a new field: `prevNode`. The `prevNode` field represents what the state of a given node was before resolving the request at hand. The `prevNode` field follows the same format as the `node`, and is omitted in the event that there was no previous state for a given node.

### Deleting a key

You can remove the `/message` key with a `DELETE` request:

```sh
curl http://127.0.0.1:2379/v2/keys/message -XDELETE
```

```json
{
    "action": "delete",
    "node": {
        "createdIndex": 3,
        "key": "/message",
        "modifiedIndex": 4
    },
    "prevNode": {
    	"key": "/message",
    	"value": "Hello etcd",
    	"modifiedIndex": 3,
    	"createdIndex": 3
    }
}
```


### Using key TTL

Keys in etcd can be set to expire after a specified number of seconds.
You can do this by setting a TTL (time to live) on the key when sending a `PUT` request:

```sh
curl http://127.0.0.1:2379/v2/keys/foo -XPUT -d value=bar -d ttl=5
```

```json
{
    "action": "set",
    "node": {
        "createdIndex": 5,
        "expiration": "2013-12-04T12:01:21.874888581-08:00",
        "key": "/foo",
        "modifiedIndex": 5,
        "ttl": 5,
        "value": "bar"
    }
}
```

Note the two new fields in response:

1. The `expiration` is the time at which this key will expire and be deleted.

2. The `ttl` is the specified time to live for the key, in seconds.

_NOTE_: Keys can only be expired by a cluster leader, so if a member gets disconnected from the cluster, its keys will not expire until it rejoins.

Now you can try to get the key by sending a `GET` request:

```sh
curl http://127.0.0.1:2379/v2/keys/foo
```

If the TTL has expired, the key will have been deleted, and you will be returned a 100.

```json
{
    "cause": "/foo",
    "errorCode": 100,
    "index": 6,
    "message": "Key not found"
}
```

The TTL can be unset to avoid expiration through update operation:

```sh
curl http://127.0.0.1:2379/v2/keys/foo -XPUT -d value=bar -d ttl= -d prevExist=true
```

```json
{
    "action": "update",
    "node": {
        "createdIndex": 5,
        "key": "/foo",
        "modifiedIndex": 6,
        "value": "bar"
    },
    "prevNode": {
        "createdIndex": 5,
        "expiration": "2013-12-04T12:01:21.874888581-08:00",
        "key": "/foo",
        "modifiedIndex": 5,
        "ttl": 3,
        "value": "bar"
    }
}
```


### Waiting for a change

We can watch for a change on a key and receive a notification by using long polling.
This also works for child keys by passing `recursive=true` in curl.

In one terminal, we send a `GET` with `wait=true` :

```sh
curl http://127.0.0.1:2379/v2/keys/foo?wait=true
```

Now we are waiting for any changes at path `/foo`.

In another terminal, we set a key `/foo` with value `bar`:

```sh
curl http://127.0.0.1:2379/v2/keys/foo -XPUT -d value=bar
```

The first terminal should get the notification and return with the same response as the set request:

```json
{
    "action": "set",
    "node": {
        "createdIndex": 7,
        "key": "/foo",
        "modifiedIndex": 7,
        "value": "bar"
    },
    "prevNode": {
        "createdIndex": 6,
        "key": "/foo",
        "modifiedIndex": 6,
        "value": "bar"
    }
}
```

However, the watch command can do more than this.
Using the index, we can watch for commands that have happened in the past.
This is useful for ensuring you don't miss events between watch commands.

Let's try to watch for the set command of index 7 again:

```sh
curl 'http://127.0.0.1:2379/v2/keys/foo?wait=true&waitIndex=7'
```

The watch command returns immediately with the same response as previously.


### Atomically Creating In-Order Keys

Using `POST` on a directory, you can create keys with key names that are created in-order.
This can be used in a variety of useful patterns, like implementing queues of keys which need to be processed in strict order.
An example use case would be ensuring clients get fair access to a mutex.

Creating an in-order key is easy:

```sh
curl http://127.0.0.1:2379/v2/keys/queue -XPOST -d value=Job1
```

```json
{
    "action": "create",
    "node": {
        "createdIndex": 6,
        "key": "/queue/6",
        "modifiedIndex": 6,
        "value": "Job1"
    }
}
```

If you create another entry some time later, it is guaranteed to have a key name that is greater than the previous key.
Also note the key names use the global etcd index, so the next key can be more than `previous + 1`.

```sh
curl http://127.0.0.1:2379/v2/keys/queue -XPOST -d value=Job2
```

```json
{
    "action": "create",
    "node": {
        "createdIndex": 29,
        "key": "/queue/29",
        "modifiedIndex": 29,
        "value": "Job2"
    }
}
```

To enumerate the in-order keys as a sorted list, use the "sorted" parameter.

```sh
curl -s 'http://127.0.0.1:2379/v2/keys/queue?recursive=true&sorted=true'
```

```json
{
    "action": "get",
    "node": {
        "createdIndex": 2,
        "dir": true,
        "key": "/queue",
        "modifiedIndex": 2,
        "nodes": [
            {
                "createdIndex": 2,
                "key": "/queue/2",
                "modifiedIndex": 2,
                "value": "Job1"
            },
            {
                "createdIndex": 3,
                "key": "/queue/3",
                "modifiedIndex": 3,
                "value": "Job2"
            }
        ]
    }
}
```


### Using a directory TTL

Like keys, directories in etcd can be set to expire after a specified number of seconds.
You can do this by setting a TTL (time to live) on a directory when it is created with a `PUT`:

```sh
curl http://127.0.0.1:2379/v2/keys/dir -XPUT -d ttl=30 -d dir=true
```

```json
{
    "action": "set",
    "node": {
        "createdIndex": 17,
        "dir": true,
        "expiration": "2013-12-11T10:37:33.689275857-08:00",
        "key": "/dir",
        "modifiedIndex": 17,
        "ttl": 30
    }
}
```

The directory's TTL can be refreshed by making an update.
You can do this by making a PUT with `prevExist=true` and a new TTL.

```sh
curl http://127.0.0.1:2379/v2/keys/dir -XPUT -d ttl=30 -d dir=true -d prevExist=true
```

Keys that are under this directory work as usual, but when the directory expires, a watcher on a key under the directory will get an expire event:

```sh
curl 'http://127.0.0.1:2379/v2/keys/dir/asdf?consistent=true&wait=true'
```

```json
{
    "action": "expire",
    "node": {
        "createdIndex": 8,
        "key": "/dir",
        "modifiedIndex": 15
    },
    "prevNode": {
        "createdIndex": 8,
        "key": "/dir",
        "dir":true,
        "modifiedIndex": 17,
        "expiration": "2013-12-11T10:39:35.689275857-08:00"
    }
}
```


### Atomic Compare-and-Swap

etcd can be used as a centralized coordination service in a cluster, and `CompareAndSwap` (CAS) is the most basic operation used to build a distributed lock service.

This command will set the value of a key only if the client-provided conditions are equal to the current conditions.

The current comparable conditions are:

1. `prevValue` - checks the previous value of the key.

2. `prevIndex` - checks the previous modifiedIndex of the key.

3. `prevExist` - checks existence of the key: if `prevExist` is true, it is an `update` request; if prevExist is `false`, it is a `create` request.

Here is a simple example.
Let's create a key-value pair first: `foo=one`.

```sh
curl http://127.0.0.1:2379/v2/keys/foo -XPUT -d value=one
```

Now let's try some invalid `CompareAndSwap` commands.

Trying to set this existing key with `prevExist=false` fails as expected:
```sh
curl http://127.0.0.1:2379/v2/keys/foo?prevExist=false -XPUT -d value=three
```

The error code explains the problem:

```json
{
    "cause": "/foo",
    "errorCode": 105,
    "index": 39776,
    "message": "Key already exists"
}
```

Now let's provide a `prevValue` parameter:

```sh
curl http://127.0.0.1:2379/v2/keys/foo?prevValue=two -XPUT -d value=three
```

This will try to compare the previous value of the key and the previous value we provided. If they are equal, the value of the key will change to three.

```json
{
    "cause": "[two != one]",
    "errorCode": 101,
    "index": 8,
    "message": "Compare failed"
}
```

which means `CompareAndSwap` failed. `cause` explains why the test failed.
Note: the condition prevIndex=0 always passes.

Let's try a valid condition:

```sh
curl http://127.0.0.1:2379/v2/keys/foo?prevValue=one -XPUT -d value=two
```

The response should be:

```json
{
    "action": "compareAndSwap",
    "node": {
        "createdIndex": 8,
        "key": "/foo",
        "modifiedIndex": 9,
        "value": "two"
    },
    "prevNode": {
    	"createdIndex": 8,
    	"key": "/foo",
    	"modifiedIndex": 8,
    	"value": "one"
    }
}
```

We successfully changed the value from "one" to "two" since we gave the correct previous value.

### Atomic Compare-and-Delete

This command will delete a key only if the client-provided conditions are equal to the current conditions.

The current comparable conditions are:

1. `prevValue` - checks the previous value of the key.

2. `prevIndex` - checks the previous modifiedIndex of the key.

Here is a simple example. Let's first create a key: `foo=one`.

```sh
curl http://127.0.0.1:2379/v2/keys/foo -XPUT -d value=one
```

Now let's try some `CompareAndDelete` commands.

Trying to delete the key with `prevValue=two` fails as expected:
```sh
curl http://127.0.0.1:2379/v2/keys/foo?prevValue=two -XDELETE
```

The error code explains the problem:

```json
{
	"errorCode": 101,
	"message": "Compare failed",
	"cause": "[two != one]",
	"index": 8
}
```

As does a `CompareAndDelete` with a mismatched `prevIndex`:

```sh
curl http://127.0.0.1:2379/v2/keys/foo?prevIndex=1 -XDELETE
```

```json
{
	"errorCode": 101,
	"message": "Compare failed",
	"cause": "[1 != 8]",
	"index": 8
}
```

And now a valid `prevValue` condition:

```sh
curl http://127.0.0.1:2379/v2/keys/foo?prevValue=one -XDELETE
```

The successful response will look something like:

```json
{
	"action": "compareAndDelete",
	"node": {
		"key": "/foo",
		"modifiedIndex": 9,
		"createdIndex": 8
	},
	"prevNode": {
		"key": "/foo",
		"value": "one",
		"modifiedIndex": 8,
		"createdIndex": 8
	}
}
```

### Creating Directories

In most cases, directories for a key are automatically created.
But there are cases where you will want to create a directory or remove one.

Creating a directory is just like a key except you cannot provide a value and must add the `dir=true` parameter.

```sh
curl http://127.0.0.1:2379/v2/keys/dir -XPUT -d dir=true
```
```json
{
    "action": "set",
    "node": {
        "createdIndex": 30,
        "dir": true,
        "key": "/dir",
        "modifiedIndex": 30
    }
}
```


### Listing a directory

In etcd we can store two types of things: keys and directories.
Keys store a single string value.
Directories store a set of keys and/or other directories.

In this example, let's first create some keys:

We already have `/foo=two` so now we'll create another one called `/foo_dir/foo` with the value of `bar`:

```sh
curl http://127.0.0.1:2379/v2/keys/foo_dir/foo -XPUT -d value=bar
```

```json
{
    "action": "set",
    "node": {
        "createdIndex": 2,
        "key": "/foo_dir/foo",
        "modifiedIndex": 2,
        "value": "bar"
    }
}
```

Now we can list the keys under root `/`:

```sh
curl http://127.0.0.1:2379/v2/keys/
```

We should see the response as an array of items:

```json
{
    "action": "get",
    "node": {
        "key": "/",
        "dir": true,
        "nodes": [
            {
                "key": "/foo_dir",
                "dir": true,
                "modifiedIndex": 2,
                "createdIndex": 2
            },
            {
                "key": "/foo",
                "value": "two",
                "modifiedIndex": 1,
                "createdIndex": 1
            }
        ]
    }
}
```

Here we can see `/foo` is a key-value pair under `/` and `/foo_dir` is a directory.
We can also recursively get all the contents under a directory by adding `recursive=true`.

```sh
curl http://127.0.0.1:2379/v2/keys/?recursive=true
```

```json
{
    "action": "get",
    "node": {
        "key": "/",
        "dir": true,
        "nodes": [
            {
                "key": "/foo_dir",
                "dir": true,
                "nodes": [
                    {
                        "key": "/foo_dir/foo",
                        "value": "bar",
                        "modifiedIndex": 2,
                        "createdIndex": 2
                    }
                ],
                "modifiedIndex": 2,
                "createdIndex": 2
            },
            {
                "key": "/foo",
                "value": "two",
                "modifiedIndex": 1,
                "createdIndex": 1
            }
        ]
    }
}
```


### Deleting a Directory

Now let's try to delete the directory `/foo_dir`.

You can remove an empty directory using the `DELETE` verb and the `dir=true` parameter.

```sh
curl 'http://127.0.0.1:2379/v2/keys/foo_dir?dir=true' -XDELETE
```
```json
{
    "action": "delete",
    "node": {
        "createdIndex": 30,
        "dir": true,
        "key": "/foo_dir",
        "modifiedIndex": 31
    },
    "prevNode": {
    	"createdIndex": 30,
    	"key": "/foo_dir",
    	"dir": true,
    	"modifiedIndex": 30
    }
}
```

To delete a directory that holds keys, you must add `recursive=true`.

```sh
curl http://127.0.0.1:2379/v2/keys/dir?recursive=true -XDELETE
```

```json
{
    "action": "delete",
    "node": {
        "createdIndex": 10,
        "dir": true,
        "key": "/dir",
        "modifiedIndex": 11
    },
    "prevNode": {
    	"createdIndex": 10,
    	"dir": true,
    	"key": "/dir",
    	"modifiedIndex": 10
    }
}
```


### Creating a hidden node

We can create a hidden key-value pair or directory by add a `_` prefix.
The hidden item will not be listed when sending a `GET` request for a directory.

First we'll add a hidden key named `/_message`:

```sh
curl http://127.0.0.1:2379/v2/keys/_message -XPUT -d value="Hello hidden world"
```

```json
{
    "action": "set",
    "node": {
        "createdIndex": 3,
        "key": "/_message",
        "modifiedIndex": 3,
        "value": "Hello hidden world"
    }
}
```

Next we'll add a regular key named `/message`:

```sh
curl http://127.0.0.1:2379/v2/keys/message -XPUT -d value="Hello world"
```

```json
{
    "action": "set",
    "node": {
        "createdIndex": 4,
        "key": "/message",
        "modifiedIndex": 4,
        "value": "Hello world"
    }
}
```

Now let's try to get a listing of keys under the root directory, `/`:

```sh
curl http://127.0.0.1:2379/v2/keys/
```

```json
{
    "action": "get",
    "node": {
        "dir": true,
        "key": "/",
        "nodes": [
            {
                "createdIndex": 2,
                "dir": true,
                "key": "/foo_dir",
                "modifiedIndex": 2
            },
            {
                "createdIndex": 4,
                "key": "/message",
                "modifiedIndex": 4,
                "value": "Hello world"
            }
        ]
    }
}
```

Here we see the `/message` key but our hidden `/_message` key is not returned.

### Setting a key from a file

You can also use etcd to store small configuration files, json documents, XML documents, etc directly.
For example you can use curl to upload a simple text file and encode it:

```
echo "Hello\nWorld" > afile.txt
curl http://127.0.0.1:2379/v2/keys/afile -XPUT --data-urlencode value@afile.txt
```

```json
{
    "action": "get",
    "node": {
        "createdIndex": 2,
        "key": "/afile",
        "modifiedIndex": 2,
        "value": "Hello\nWorld\n"
    }
}
```

### Read Consistency

#### Read from the Master

Followers in a cluster can be behind the leader in their copy of the keyspace.
If your application wants or needs the most up-to-date version of a key then it should ensure it reads from the current leader.
By using the `consistent=true` flag in your GET requests, etcd will make sure you are talking to the current master.

As an example of how a member can be behind the leader let's start with a three member cluster: L, F1, and F2.
A client makes a write to L and F1 acknowledges the request.
The client is told the write was successful and the keyspace is updated.
Meanwhile F2 has partitioned from the network and will have an out-of-date version of the keyspace until the partition resolves.
Since F2 missed the most recent write, a client reading from F2 will have an out-of-date version of the keyspace.

Implementation notes on `consistent=true`: If the leader you are talking to is
partitioned it will be unable to determine if it is not currently the master.
In a later version we will provide a mechanism to set an upperbound of time
that the current master can be unable to contact the quorom and still serve
reads.

### Read Linearization

If you want a read that is fully linearized you can use a `quorum=true` GET.
The read will take a very similar path to a write and will have a similar
speed. If you are unsure if you need this feature feel free to email etcd-dev
for advice.

## Statistics

An etcd cluster keeps track of a number of statistics including latency, bandwidth and uptime.
These are exposed via the statistics endpoint to understand the internal health of a cluster.

### Leader Statistics

The leader has a view of the entire cluster and keeps track of two interesting statistics: latency to each peer in the cluster, and the number of failed and successful Raft RPC requests.
You can grab these statistics from the `/v2/stats/leader` endpoint:

```sh
curl http://127.0.0.1:2379/v2/stats/leader
```

```json
{
    "id": "2c7d3e0b8627375b",
    "leaderInfo": {
        "leader": "8a69d5f6b7814500",
        "startTime": "2014-10-24T13:15:51.184719899-07:00",
        "uptime": "7m17.859616962s"
    },
    "name": "infra1",
    "recvAppendRequestCnt": 3949,
    "recvBandwidthRate": 561.5729321100841,
    "recvPkgRate": 9.008227977383449,
    "sendAppendRequestCnt": 0,
    "startTime": "2014-10-24T13:15:50.070369454-07:00",
    "state": "StateFollower"
}
```


### Self Statistics

Each node keeps a number of internal statistics:

- `id`: the unique identifier for the member
- `leaderInfo.leader`: id of the current leader member
- `leaderInfo.uptime`: amount of time the leader has been leader
- `name`: this member's name
- `recvAppendRequestCnt`: number of append requests this node has processed
- `recvBandwidthRate`: number of bytes per second this node is receiving (follower only)
- `recvPkgRate`: number of requests per second this node is receiving (follower only)
- `sendAppendRequestCnt`: number of requests that this node has sent
- `sendBandwidthRate`: number of bytes per second this node is receiving (leader only). This value is undefined on single member clusters.
- `sendPkgRate`: number of requests per second this node is receiving (leader only). This value is undefined on single member clusters.
- `state`: either leader or follower
- `startTime`: the time when this node was started

This is an example response from a follower member:

```sh
curl http://127.0.0.1:2379/v2/stats/self
```

```json
{
    "id": "eca0338f4ea31566",
    "leaderInfo": {
        "leader": "8a69d5f6b7814500",
        "startTime": "2014-10-24T13:15:51.186620747-07:00",
        "uptime": "10m59.322358947s"
    },
    "name": "node3",
    "recvAppendRequestCnt": 5944,
    "recvBandwidthRate": 570.6254930219969,
    "recvPkgRate": 9.00892789741075,
    "sendAppendRequestCnt": 0,
    "startTime": "2014-10-24T13:15:50.072007085-07:00",
    "state": "StateFollower"
}
```

And this is an example response from a leader member:

```sh
curl http://127.0.0.1:2379/v2/stats/self
```

```json
{
    "id": "eca0338f4ea31566",
    "leaderInfo": {
        "leader": "8a69d5f6b7814500",
        "startTime": "2014-10-24T13:15:51.186620747-07:00",
        "uptime": "10m47.012122091s"
    },
    "name": "node3",
    "recvAppendRequestCnt": 5835,
    "recvBandwidthRate": 584.1485698657176,
    "recvPkgRate": 9.17390765395709,
    "sendAppendRequestCnt": 0,
    "startTime": "2014-10-24T13:15:50.072007085-07:00",
    "state": "StateFollower"
}
```


### Store Statistics

The store statistics include information about the operations that this node has handled.

Operations that modify the store's state like create, delete, set and update are seen by the entire cluster and the number will increase on all nodes.
Operations like get and watch are node local and will only be seen on this node.

```sh
curl http://127.0.0.1:2379/v2/stats/store
```

```json
{
    "compareAndSwapFail": 0,
    "compareAndSwapSuccess": 0,
    "createFail": 0,
    "createSuccess": 2,
    "deleteFail": 0,
    "deleteSuccess": 0,
    "expireCount": 0,
    "getsFail": 4,
    "getsSuccess": 75,
    "setsFail": 2,
    "setsSuccess": 4,
    "updateFail": 0,
    "updateSuccess": 0,
    "watchers": 0
}
```

## Cluster Config

See the [other etcd APIs][other-apis] for details on the cluster management.

[other-apis]: https://github.com/coreos/etcd/blob/master/Documentation/0.5/other_apis.md
