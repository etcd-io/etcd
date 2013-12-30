# etcd

README version 0.2.0

[![Build Status](https://travis-ci.org/coreos/etcd.png)](https://travis-ci.org/coreos/etcd)

A highly-available key value store for shared configuration and service discovery.
etcd is inspired by zookeeper and doozer, with a focus on:

* Simple: curl'able user facing API (HTTP+JSON)
* Secure: optional SSL client cert authentication
* Fast: benchmarked 1000s of writes/s per instance
* Reliable: Properly distributed using Raft

Etcd is written in Go and uses the [Raft][raft] consensus algorithm to manage a highly-available replicated log.

See [etcdctl][etcdctl] for a simple command line client.
Or feel free to just use curl, as in the examples below.

[raft]: https://github.com/coreos/go-raft
[etcdctl]: http://github.com/coreos/etcdctl/

## Contact

- Mailing list: http://coreos.com/lists/etcd-dev/
- IRC: #coreos on irc.freenode.net
- Planning/Roadmap: https://trello.com/b/OiEbU547/etcd
- Bugs: https://github.com/coreos/etcd/issues


## Getting Started

### Getting etcd

The latest release and setup instructions are available at [Github][github-release].

[github-release]: https://github.com/coreos/etcd/releases/


### Building

You can build etcd from source:

```sh
git clone https://github.com/coreos/etcd
cd etcd
./build
```

This will generate a binary in the base directory called `./etcd`.

_NOTE_: you need go 1.1+. Please check your installation with

```
go version
```


### Running a single machine

These examples will use a single machine cluster to show you the basics of the etcd REST API.
Let's start etcd:

```sh
./etcd -data-dir machine0 -name machine0
```

This will bring up etcd listening on port 4001 for client communication and on port 7001 for server-to-server communication.
The `-data-dir machine0` argument tells etcd to write machine configuration, logs and snapshots to the `./machine0/` directory.
The `-name machine` tells the rest of the cluster that this machine is named machine0.



## Usage

### Setting the value to a key

Let’s set the first key-value pair to the datastore.
In this case the key is `/message` and the value is `Hello world`.

```sh
curl -L http://127.0.0.1:4001/v2/keys/message -X PUT -d value="Hello world"
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

This response contains four fields.
We will introduce three more fields as we try more commands.

1. The action of the request; we set the value via a `PUT` request, thus the action is `set`.

2. The key of the request; we set `/message` to `Hello world`, so the key field is `/message`.
We use a file system like structure to represent the key-value pairs so each key starts with `/`.

3. The current value of the key; we set the value to`Hello world`.

4. Modified Index is a unique, monotonically incrementing index created for each change to etcd.
Requests that change the index include `set`, `delete`, `update`, `create` and `compareAndSwap`.
Since the `get` and `watch` commands do not change state in the store, they do not change the index.
You may notice that in this example the index is `2` even though it is the first request you sent to the server.
This is because there are internal commands that also change the state like adding and syncing servers.

### Response Headers

etcd includes a few HTTP headers that provide global information about the etcd cluster that serviced a request:

```
X-Etcd-Index: 35
X-Raft-Index: 5398
X-Raft-Term: 0
```

- `X-Etcd-Index` is the current etcd index as explained above.
- `X-Raft-Index` is similar to the etcd index but is for the underlying raft protocol
- `X-Raft-Term` this number will increase when an etcd master election happens. If this number is increasing rapdily you may need to tune the election timeout. See the [tuning][tuning] section for details.

[tuning]: #tuning

### Get the value of a key

We can get the value that we just set in `/message` by issuing a `GET` request:

```sh
curl -L http://127.0.0.1:4001/v2/keys/message
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
curl -L http://127.0.0.1:4001/v2/keys/message -XPUT -d value="Hello etcd"
```

```json
{
    "action": "set",
    "node": {
        "createdIndex": 3,
        "key": "/message",
        "modifiedIndex": 3,
        "prevValue": "Hello world",
        "value": "Hello etcd"
    }
}
```

Notice that `node.prevValue` is set to the previous value of the key - `Hello world`.
It is useful when you want to atomically set a value to a key and get its old value.


### Deleting a key

You can remove the `/message` key with a `DELETE` request:

```sh
curl -L http://127.0.0.1:4001/v2/keys/message -XDELETE
```

```json
{
    "action": "delete",
    "node": {
        "createdIndex": 3,
        "key": "/message",
        "modifiedIndex": 4,
        "prevValue": "Hello etcd"
    }
}
```


### Using key TTL

Keys in etcd can be set to expire after a specified number of seconds.
You can do this by setting a TTL (time to live) on the key when send a `PUT` request:

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar -d ttl=5
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

1. The `expiration` is the time that this key will expire and be deleted.

2. The `ttl` is the time to live for the key, in seconds.

_NOTE_: Keys can only be expired by a cluster leader so if a machine gets disconnected from the cluster, its keys will not expire until it rejoins.

Now you can try to get the key by sending a `GET` request:

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo
```

If the TTL has expired, the key will be deleted, and you will be returned a 100.

```json
{
    "cause": "/foo",
    "errorCode": 100,
    "index": 6,
    "message": "Key Not Found"
}
```

### Waiting for a change

We can watch for a change on a key and receive a notification by using long polling.
This also works for child keys by passing `recursive=true` in curl.

In one terminal, we send a get request with `wait=true` :

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo?wait=true
```

Now we are waiting for any changes at path `/foo`.

In another terminal, we set a key `/foo` with value `bar`:

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar
```

The first terminal should get the notification and return with the same response as the set request.

```json
{
    "action": "set",
    "node": {
        "createdIndex": 7,
        "key": "/foo",
        "modifiedIndex": 7,
        "value": "bar"
    }
}
```

However, the watch command can do more than this.
Using the the index we can watch for commands that has happened in the past.
This is useful for ensuring you don't miss events between watch commands.

Let's try to watch for the set command of index 7 again:

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo?wait=true\&waitIndex=7
```

The watch command returns immediately with the same response as previous.


### Atomically Creating In-Order Keys

Using the `POST` on a directory you can create keys with key names that are created in-order.
This can be used in a variety of useful patterns like implementing queues of keys that need to be processed in strict order.
An example use case is the [locking module][lockmod] which uses it to ensure clients get fair access to a mutex.

Creating an in-order key is easy

```sh
curl -X POST http://127.0.0.1:4001/v2/keys/queue -d value=Job1
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

If you create another entry some time later it is guaranteed to have a key name that is greater than the previous key.
Also note the key names use the global etcd index so the next key can be more than `previous + 1`.

```sh
curl -X POST http://127.0.0.1:4001/v2/keys/queue -d value=Job2
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
curl -s -X GET 'http://127.0.0.1:4001/v2/keys/queue?recursive=true&sorted=true'
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

[lockmod]: #lock


### Using a directory TTL

Like keys, directories in etcd can be set to expire after a specified number of seconds.
You can do this by setting a TTL (time to live) on a directory when it is created with a `PUT`:

```sh
curl -L http://127.0.0.1:4001/v2/keys/dir -XPUT -d ttl=30 -d dir=true
```

```json
{
    "action": "set",
    "node": {
        "createdIndex": 17,
        "dir": true,
        "expiration": "2013-12-11T10:37:33.689275857-08:00",
        "key": "/newdir",
        "modifiedIndex": 17,
        "ttl": 30
    }
}
```

The directories TTL can be refreshed by making an update.
You can do this by making a PUT with `prevExist=true` and a new TTL.

```sh
curl -L http://127.0.0.1:4001/v2/keys/dir -XPUT -d ttl=30 -d dir=true -d prevExist=true
```

Keys that are under this directory work as usual, but when the directory expires a watcher on a key under the directory will get an expire event:

```sh
curl -X GET http://127.0.0.1:4001/v2/keys/dir/asdf\?consistent\=true\&wait\=true
```

```json
{
    "action": "expire",
    "node": {
        "createdIndex": 8,
        "key": "/dir",
        "modifiedIndex": 15
    }
}
```


### Atomic Compare-and-Swap (CAS)

Etcd can be used as a centralized coordination service in a cluster and `CompareAndSwap` is the most basic operation used to build a distributed lock service.

This command will set the value of a key only if the client-provided conditions are equal to the current conditions.

The current comparable conditions are:

1. `prevValue` - checks the previous value of the key.

2. `prevIndex` - checks the previous index of the key.

3. `prevExist` - checks existence of the key: if `prevExist` is true, it is a  `update` request; if prevExist is `false`, it is a `create` request.

Here is a simple example.
Let's create a key-value pair first: `foo=one`.

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo -XPUT -d value=one
```

Let's try some invalid `CompareAndSwap` commands first.

Trying to set this existing key with `prevExist=false` fails as expected:
```sh
curl -L http://127.0.0.1:4001/v2/keys/foo?prevExist=false -XPUT -d value=three
```

The error code explains the problem:

```json
{
    "cause": "/foo",
    "errorCode": 105,
    "index": 39776,
    "message": "Already exists"
}
```

Now lets provide a `prevValue` parameter:

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo?prevValue=two -XPUT -d value=three
```

This will try to compare the previous value of the key and the previous value we provided. If they are equal, the value of the key will change to three.

```json
{
    "cause": "[two != one] [0 != 8]",
    "errorCode": 101,
    "index": 8,
    "message": "Test Failed"
}
```

which means `CompareAndSwap` failed.

Let's try a valid condition:

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo?prevValue=one -XPUT -d value=two
```

The response should be

```json
{
    "action": "compareAndSwap",
    "node": {
        "createdIndex": 8,
        "key": "/foo",
        "modifiedIndex": 9,
        "prevValue": "one",
        "value": "two"
    }
}
```

We successfully changed the value from "one" to "two" since we gave the correct previous value.

### Creating Directories

In most cases directories for a key are automatically created.
But, there are cases where you will want to create a directory or remove one.

Creating a directory is just like a key only you cannot provide a value and must add the `dir=true` parameter.

```sh
curl -L http://127.0.0.1:4001/v2/keys/dir -XPUT -d dir=true
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
curl -L http://127.0.0.1:4001/v2/keys/foo_dir/foo -XPUT -d value=bar
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
curl -L http://127.0.0.1:4001/v2/keys/
```

We should see the response as an array of items:

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
            }
        ]
    }
}
```

Here we can see `/foo` is a key-value pair under `/` and `/foo_dir` is a directory.
We can also recursively get all the contents under a directory by adding `recursive=true`.

```sh
curl -L http://127.0.0.1:4001/v2/keys/?recursive=true
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
                "modifiedIndex": 2,
                "nodes": [
                    {
                        "createdIndex": 2,
                        "key": "/foo_dir/foo",
                        "modifiedIndex": 2,
                        "value": "bar"
                    }
                ]
            }
        ]
    }
}
```


### Deleting a Directory

Now let's try to delete the directory `/foo_dir`.

You can remove an empty directory using the `DELETE` verb and the `dir=true` parameter.

```sh
curl -L -X DELETE 'http://127.0.0.1:4001/v2/keys/dir?dir=true'
```
```json
{
    "action": "delete",
    "node": {
        "createdIndex": 30,
        "dir": true,
        "key": "/dir",
        "modifiedIndex": 31
    }
}
```

To delete a directory that holds keys, you must add `recursive=true`.

```sh
curl -L http://127.0.0.1:4001/v2/keys/dir?recursive=true -XDELETE
```

```json
{
    "action": "delete",
    "node": {
        "createdIndex": 10,
        "dir": true,
        "key": "/dir",
        "modifiedIndex": 11
    }
}
```


### Creating a hidden node

We can create a hidden key-value pair or directory by add a `_` prefix.
The hidden item will not be listed when sending a `GET` request for a directory.

First we'll add a hidden key named `/_message`:

```sh
curl -L http://127.0.0.1:4001/v2/keys/_message -XPUT -d value="Hello hidden world"
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
curl -L http://127.0.0.1:4001/v2/keys/message -XPUT -d value="Hello world"
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
curl -L http://127.0.0.1:4001/v2/keys/
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

## Advanced Usage

### Transport security with HTTPS

Etcd supports SSL/TLS and client cert authentication for clients to server, as well as server to server communication.

First, you need to have a CA cert `clientCA.crt` and signed key pair `client.crt`, `client.key`.
This site has a good reference for how to generate self-signed key pairs:
http://www.g-loaded.eu/2005/11/10/be-your-own-ca/

For testing you can use the certificates in the `fixtures/ca` directory.

Let's configure etcd to use this keypair:

```sh
./etcd -f -name machine0 -data-dir machine0 -cert-file=./fixtures/ca/server.crt -key-file=./fixtures/ca/server.key.insecure
```

There are a few new options we're using:

* `-f` - forces a new machine configuration, even if an existing configuration is found. (WARNING: data loss!)
* `-cert-file` and `-key-file` specify the location of the cert and key files to be used for for transport layer security between the client and server.

You can now test the configuration using HTTPS:

```sh
curl --cacert ./fixtures/ca/server-chain.pem https://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar -v
```

You should be able to see the handshake succeed.

**OSX 10.9+ Users**: curl 7.30.0 on OSX 10.9+ doesn't understand certificates passed in on the command line.
Instead you must import the dummy ca.crt directly into the keychain or add the `-k` flag to curl to ignore errors.
If you want to test without the `-k` flag run `open ./fixtures/ca/ca.crt` and follow the prompts.
Please remove this certificate after you are done testing!
If you know of a workaround let us know.

```
...
SSLv3, TLS handshake, Finished (20):
...
```

And also the response from the etcd server:

```json
{
    "action": "set",
    "key": "/foo",
    "modifiedIndex": 3,
    "prevValue": "bar",
    "value": "bar"
}
```


### Authentication with HTTPS client certificates

We can also do authentication using CA certs.
The clients will provide their cert to the server and the server will check whether the cert is signed by the CA and decide whether to serve the request.

```sh
./etcd -f -name machine0 -data-dir machine0 -ca-file=./fixtures/ca/ca.crt -cert-file=./fixtures/ca/server.crt -key-file=./fixtures/ca/server.key.insecure
```

```-ca-file``` is the path to the CA cert.

Try the same request to this server:

```sh
curl --cacert ./fixtures/ca/server-chain.pem https://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar -v
```

The request should be rejected by the server.

```
...
routines:SSL3_READ_BYTES:sslv3 alert bad certificate
...
```

We need to give the CA signed cert to the server.

```sh
curl --key ./fixtures/ca/server2.key.insecure --cert ./fixtures/ca/server2.crt --cacert ./fixtures/ca/server-chain.pem -L https://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar -v
```

You should able to see:

```
...
SSLv3, TLS handshake, CERT verify (15):
...
TLS handshake, Finished (20)
```

And also the response from the server:

```json
{
    "action": "set",
    "node": {
        "createdIndex": 12,
        "key": "/foo",
        "modifiedIndex": 12,
        "prevValue": "two",
        "value": "bar"
    }
}
```


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

Let's join two more machines to this cluster using the `-peers` argument:

```sh
./etcd -peer-addr 127.0.0.1:7002 -addr 127.0.0.1:4002 -peers 127.0.0.1:7001 -data-dir machines/machine2 -name machine2
./etcd -peer-addr 127.0.0.1:7003 -addr 127.0.0.1:4003 -peers 127.0.0.1:7001 -data-dir machines/machine3 -name machine3
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

## Modules

etcd has a number of modules that are built on top of the core etcd API.
These modules provide things like dashboards, locks and leader election.

### Dashboard

An HTML dashboard can be found at `http://127.0.0.1:4001/mod/dashboard/`

### Lock

The Lock module implements a fair lock that can be used when lots of clients want access to a single resource.
A lock can be associated with a value.
The value is unique so if a lock tries to request a value that is already queued for a lock then it will find it and watch until that value obtains the lock.
If you lock the same value on a key from two separate curl sessions they'll both return at the same time.

Here's the API:

**Acquire a lock (with no value) for "customer1"**

```sh
curl -X POST http://127.0.0.1:4001/mod/v2/lock/customer1?ttl=60
```

**Acquire a lock for "customer1" that is associated with the value "bar"**

```sh
curl -X POST http://127.0.0.1:4001/mod/v2/lock/customer1?ttl=60 -d value=bar
```

**Renew the TTL on the "customer1" lock for index 2**

```sh
curl -X PUT http://127.0.0.1:4001/mod/v2/lock/customer1?ttl=60 -d index=2
```

**Renew the TTL on the "customer1" lock for value "customer1"**

```sh
curl -X PUT http://127.0.0.1:4001/mod/v2/lock/customer1?ttl=60 -d value=bar
```

**Retrieve the current value for the "customer1" lock.**

```sh
curl http://127.0.0.1:4001/mod/v2/lock/customer1
```

**Retrieve the current index for the "customer1" lock**

```sh
curl http://127.0.0.1:4001/mod/v2/lock/customer1?field=index
```

**Delete the "customer1" lock with the index 2**

```sh
curl -X DELETE http://127.0.0.1:4001/mod/v2/lock/customer1?index=2
```

**Delete the "customer1" lock with the value "bar"**

```sh
curl -X DELETE http://127.0.0.1:4001/mod/v2/lock/customer1?value=bar
```


### Leader Election

The Leader Election module wraps the Lock module to allow clients to come to consensus on a single value.
This is useful when you want one server to process at a time but allow other servers to fail over.
The API is similar to the Lock module but is limited to simple strings values.

Here's the API:

**Attempt to set a value for the "order_processing" leader key:**

```sh
curl -X PUT http://127.0.0.1:4001/mod/v2/leader/order_processing?ttl=60 -d name=myserver1.foo.com
```

**Retrieve the current value for the "order_processing" leader key:**

```sh
curl http://127.0.0.1:4001/mod/v2/leader/order_processing
myserver1.foo.com
```

**Remove a value from the "order_processing" leader key:**

```sh
curl -X DELETE http://127.0.0.1:4001/mod/v2/leader/order_processing?name=myserver1.foo.com
```

If multiple clients attempt to set the value for a key then only one will succeed.
The other clients will hang until the current value is removed because of TTL or because of a `DELETE` operation.
Multiple clients can submit the same value and will all be notified when that value succeeds.

To update the TTL of a value simply reissue the same `PUT` command that you used to set the value.


## Contributing

See [CONTRIBUTING](https://github.com/coreos/etcd/blob/master/CONTRIBUTING.md) for details on submitting patches and contacting developers via IRC and mailing lists.


## Libraries and Tools

**Tools**

- [etcdctl](https://github.com/coreos/etcdctl) - A command line client for etcd

**Go libraries**

- [go-etcd](https://github.com/coreos/go-etcd)

**Java libraries**

- [justinsb/jetcd](https://github.com/justinsb/jetcd)
- [diwakergupta/jetcd](https://github.com/diwakergupta/jetcd)

**Python libraries**

- [transitorykris/etcd-py](https://github.com/transitorykris/etcd-py)
- [jplana/python-etcd](https://github.com/jplana/python-etcd)
- [russellhaering/txetcd](https://github.com/russellhaering/txetcd) - a Twisted Python library

**Node libraries**

- [stianeikeland/node-etcd](https://github.com/stianeikeland/node-etcd)

**Ruby libraries**

- [iconara/etcd-rb](https://github.com/iconara/etcd-rb)
- [jpfuentes2/etcd-ruby](https://github.com/jpfuentes2/etcd-ruby)
- [ranjib/etcd-ruby](https://github.com/ranjib/etcd-ruby)

**C libraries**

- [jdarcy/etcd-api](https://github.com/jdarcy/etcd-api)

**Clojure libraries**

- [aterreno/etcd-clojure](https://github.com/aterreno/etcd-clojure)
- [dwwoelfel/cetcd](https://github.com/dwwoelfel/cetcd)
- [rthomas/clj-etcd](https://github.com/rthomas/clj-etcd)

**Erlang libraries**

- [marshall-lee/etcd.erl](https://github.com/marshall-lee/etcd.erl)

**Chef Integration**

- [coderanger/etcd-chef](https://github.com/coderanger/etcd-chef)

**Chef Cookbook**

- [spheromak/etcd-cookbook](https://github.com/spheromak/etcd-cookbook)

**BOSH Releases**

- [cloudfoundry-community/etcd-boshrelease](https://github.com/cloudfoundry-community/etcd-boshrelease)
- [cloudfoundry/cf-release](https://github.com/cloudfoundry/cf-release/tree/master/jobs/etcd)

**Projects using etcd**

- [binocarlos/yoda](https://github.com/binocarlos/yoda) - etcd + ZeroMQ
- [calavera/active-proxy](https://github.com/calavera/active-proxy) - HTTP Proxy configured with etcd
- [derekchiang/etcdplus](https://github.com/derekchiang/etcdplus) - A set of distributed synchronization primitives built upon etcd
- [go-discover](https://github.com/flynn/go-discover) - service discovery in Go
- [gleicon/goreman](https://github.com/gleicon/goreman/tree/etcd) - Branch of the Go Foreman clone with etcd support
- [garethr/hiera-etcd](https://github.com/garethr/hiera-etcd) - Puppet hiera backend using etcd
- [mattn/etcd-vim](https://github.com/mattn/etcd-vim) - SET and GET keys from inside vim
- [mattn/etcdenv](https://github.com/mattn/etcdenv) - "env" shebang with etcd integration
- [kelseyhightower/confd](https://github.com/kelseyhightower/confd) - Manage local app config files using templates and data from etcd


## FAQ

### What size cluster should I use?

Every command the client sends to the master is broadcast to all of the followers.
The command is not committed until the majority of the cluster peers receive that command.

Because of this majority voting property, the ideal cluster should be kept small to keep speed up and be made up of an odd number of peers.

Odd numbers are good because if you have 8 peers the majority will be 5 and if you have 9 peers the majority will still be 5.
The result is that an 8 peer cluster can tolerate 3 peer failures and a 9 peer cluster can tolerate 4 machine failures.
And in the best case when all 9 peers are responding the cluster will perform at the speed of the fastest 5 machines.


### Why SSLv3 alert handshake failure when using SSL client auth?

The `crypto/tls` package of `golang` checks the key usage of the certificate public key before using it.
To use the certificate public key to do client auth, we need to add `clientAuth` to `Extended Key Usage` when creating the certificate public key.

Here is how to do it:

Add the following section to your openssl.cnf:

```
[ ssl_client ]
...
  extendedKeyUsage = clientAuth
...
```

When creating the cert be sure to reference it in the `-extensions` flag:

```
openssl ca -config openssl.cnf -policy policy_anything -extensions ssl_client -out certs/machine.crt -infiles machine.csr
```

### Tuning

The default settings in etcd should work well for installations on a local network where the average network latency is low.
However, when using etcd across multiple data centers or over networks with high latency you may need to tweak the heartbeat and election timeout settings.

The underlying distributed consensus protocol relies on two separate timeouts to ensure that nodes can handoff leadership if one stalls or goes offline.
The first timeout is called the *Heartbeat Timeout*.
This is the frequency with which the leader will notify followers that it is still the leader.
etcd batches commands together for higher throughput so this heartbeat timeout is also a delay for how long it takes for commands to be committed.
By default, etcd uses a `50ms` heartbeat timeout.

The second timeout is the *Election Timeout*.
This timeout is how long a follower node will go without hearing a heartbeat before attempting to become leader itself.
By default, etcd uses a `200ms` election timeout.

Adjusting these values is a trade off.
Lowering the heartbeat timeout will cause individual commands to be committed faster but it will lower the overall throughput of etcd.
If your etcd instances have low utilization then lowering the heartbeat timeout can improve your command response time.

The election timeout should be set based on the heartbeat timeout and your network ping time between nodes.
Election timeouts should be at least 10 times your ping time so it can account for variance in your network.
For example, if the ping time between your nodes is 10ms then you should have at least a 100ms election timeout.

You should also set your election timeout to at least 4 to 5 times your heartbeat timeout to account for variance in leader replication.
For a heartbeat timeout of 50ms you should set your election timeout to at least 200ms - 250ms.

You can override the default values on the command line:

```sh
# Command line arguments:
$ etcd -peer-heartbeat-timeout=100 -peer-election-timeout=500

# Environment variables:
$ ETCD_PEER_HEARTBEAT_TIMEOUT=100 ETCD_PEER_ELECTION_TIMEOUT=500 etcd
```

Or you can set the values within the configuration file:

```toml
[peer]
heartbeat_timeout = 100
election_timeout = 100
```

The values are specified in milliseconds.


## Project Details

### Versioning

#### Service Versioning

etcd uses [semantic versioning][semver].
New minor versions may add additional features to the API.

You can get the version of etcd by issuing a request to /version:

```sh
curl -L http://127.0.0.1:4001/version
```

[semver]: http://semver.org/

#### API Versioning

Clients are encouraged to use the `v2` API. The `v1` API will not change.

The `v2` API responses should not change after the 0.2.0 release but new features will be added over time.

During the pre-v1.0.0 series of releases we may break the API as we fix bugs and get feedback.

### License

etcd is under the Apache 2.0 license. See the [LICENSE][license] file for details.

[license]: https://github.com/coreos/etcd/blob/master/LICENSE
