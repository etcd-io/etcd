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
[etcdctl]: http://coreos.com/docs/etcdctl/

## Contact

- Mailing list: http://coreos.com/lists/etcd-dev/
- IRC: #coreos on irc.freenode.net
- Planning/Roadmap: https://trello.com/b/OiEbU547/etcd
- Bugs: https://github.com/coreos/etcd/issues


## Getting Started

### Getting etcd

The latest release is available as a binary at [Github][github-release].

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


### Running a single node

These examples will use a single node cluster to show you the basics of the etcd REST API.
Let's start etcd:

```sh
./etcd -data-dir node0 -name node0
```

This will bring up an etcd node listening on port 4001 for client communication and on port 7001 for server-to-server communication.
The `-data-dir node0` argument tells etcd to write node configuration, logs and snapshots to the `./node0/` directory.
The `-name node0` tells the rest of the cluster that this node is named node0.



## Usage

### Setting the value to a key

Let’s set the first key-value pair to the node.
In this case the key is `/message` and the value is `Hello world`.

```sh
curl -L http://127.0.0.1:4001/v2/keys/message -X PUT -d value="Hello world"
```

```json
{"action":"set","key":"/message","value":"Hello world","modifiedIndex":2}
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


### Get the value of a key

We can get the value that we just set in `/message` by issuing a `GET` request:

```sh
curl -L http://127.0.0.1:4001/v2/keys/message
```

```json
{"action":"get","key":"/message","value":"Hello world","modifiedIndex":2}
```


### Changing the value of a key

You can change the value of `/message` from `Hello world` to `Hello etcd` with another `PUT` request to the key:

```sh
curl -L http://127.0.0.1:4001/v1/keys/message -XPUT -d value="Hello etcd"
```

```json
{"action":"set","key":"/message","prevValue":"Hello world","value":"Hello etcd","index":3}
```

Notice that the `prevValue` is set to the previous value of the key - `Hello world`.
It is useful when you want to atomically set a value to a key and get its old value.


### Deleting a key

You can remove the `/message` key with a `DELETE` request:

```sh
curl -L http://127.0.0.1:4001/v2/keys/message -XDELETE
```

```json
{"action":"delete","key":"/message","prevValue":"Hello etcd","modifiedIndex":4}
```


### Using key TTL

Keys in etcd can be set to expire after a specified number of seconds.
You can do this by setting a TTL (time to live) on the key when send a `PUT` request:

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar -d ttl=5
```

```json
{"action":"set","key":"/foo","value":"bar","expiration":"2013-11-12T20:21:22.629352334-05:00","ttl":5,"modifiedIndex":5}
```

Note the two new fields in response:

1. The `expiration` is the time that this key will expire and be deleted.

2. The `ttl` is the time to live for the key, in seconds.

_NOTE_: Keys can only be expired by a cluster leader so if a node gets disconnected from the cluster, its keys will not expire until it rejoins.

Now you can try to get the key by sending a `GET` request:

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo
```

If the TTL has expired, the key will be deleted, and you will be returned a 100.

```json
{"errorCode":100,"message":"Key Not Found","cause":"/foo","index":6}
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
{"action":"set","key":"/foo","value":"bar","modifiedIndex":7}
```

However, the watch command can do more than this.
Using the the index we can watch for commands that has happened in the past.
This is useful for ensuring you don't miss events between watch commands.

Let's try to watch for the set command of index 7 again:

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo?wait=true\&waitIndex=7
```

The watch command returns immediately with the same response as previous.


### Atomic Compare-and-Swap (CAS)

Etcd can be used as a centralized coordination service in a cluster and `CompareAndSwap` is the most basic operation to build distributed lock service. 

This command will set the value of a key only if the client-provided conditions are equal to the current conditions. 

The current comparable conditions are:

1. `prevValue` - checks the previous value of the key.

2. `prevIndex` - checks the previous index of the key.

3. `prevExist` - checks existence of the key: if `prevExist` is true, it is a  `update` request; if prevExist is `false`, it is a `create` request.

Here is a simple example.
Let's create a key-value pair first: `foo=one`.

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo -XPUT -d value=one
```

Let's try an invalid `CompareAndSwap` command first.
We can provide the `prevValue` parameter to the set command to make it a `CompareAndSwap` command.

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo?prevValue=two -XPUT -d value=three
```

This will try to compare the previous value of the key and the previous value we provided. If they are equal, the value of the key will change to three.

```json
{"errorCode":101,"message":"Test Failed","cause":"[two != one] [0 != 8]","index":8}
```

which means `CompareAndSwap` failed.

Let's try a valid condition:

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo?prevValue=one -XPUT -d value=two
```

The response should be

```json
{"action":"compareAndSwap","key":"/foo","prevValue":"one","value":"two","modifiedIndex":9}
```

We successfully changed the value from “one” to “two” since we gave the correct previous value.


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
{"action":"set","key":"/foo_dir/foo","value":"bar","modifiedIndex":10}
```

Now we can list the keys under root `/`:

```sh
curl -L http://127.0.0.1:4001/v2/keys/
```

We should see the response as an array of items:

```json
{"action":"get","key":"/","dir":true,"kvs":[{"key":"/foo","value":"two","modifiedIndex":9},{"key":"/foo_dir","dir":true,"modifiedIndex":10}],"modifiedIndex":0}
```

Here we can see `/foo` is a key-value pair under `/` and `/foo_dir` is a directory.
We can also recursively get all the contents under a directory by adding `recursive=true`.

```sh
curl -L http://127.0.0.1:4001/v2/keys/?recursive=true
```

```json
{"action":"get","key":"/","dir":true,"kvs":[{"key":"/foo","value":"two","modifiedIndex":9},{"key":"/foo_dir","dir":true,"kvs":[{"key":"/foo_dir/foo","value":"bar","modifiedIndex":10}],"modifiedIndex":10}],"modifiedIndex":0}
```


### Deleting a directory

Now let's try to delete the directory `/foo_dir`.

To delete a directory, we must add `recursive=true`.

```sh
curl -L http://127.0.0.1:4001/v2/keys/foo_dir?recursive=true -XDELETE
```

```json
{"action":"delete","key":"/foo_dir","dir":true,"modifiedIndex":11}
```


### Creating a hidden node

We can create a hidden key-value pair or directory by add a `_` prefix.
The hidden item will not be listed when sending a `GET` request for a directory.

First we'll add a hidden key named `/_message`:

```sh
curl -L http://127.0.0.1:4001/v2/keys/_message -XPUT -d value="Hello hidden world"
```

```json
{"action":"set","key":"/_message","value":"Hello hidden world","modifiedIndex":12}
```


Next we'll add a regular key named `/message`:

```sh
curl -L http://127.0.0.1:4001/v2/keys/message -XPUT -d value="Hello world"
```

```json
{"action":"set","key":"/message","value":"Hello world","modifiedIndex":13}
```

Now let's try to get a listing of keys under the root directory, `/`:

```sh
curl -L http://127.0.0.1:4001/v2/keys/
```

```json
{"action":"get","key":"/","dir":true,"kvs":[{"key":"/foo","value":"two","modifiedIndex":9},{"key":"/message","value":"Hello world","modifiedIndex":13}],"modifiedIndex":0}
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
./etcd -f -name node0 -data-dir node0 -cert-file=./fixtures/ca/server.crt -key-file=./fixtures/ca/server.key.insecure
```

There are a few new options we're using:

* `-f` - forces a new node configuration, even if an existing configuration is found. (WARNING: data loss!)
* `-cert-file` and `-key-file` specify the location of the cert and key files to be used for for transport layer security between the client and server.

You can now test the configuration using HTTPS:

```sh
curl --cacert ./fixtures/ca/server-chain.pem https://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar -v
```

You should be able to see the handshake succeed.

```
...
SSLv3, TLS handshake, Finished (20):
...
```

And also the response from the etcd server:

```json
{"action":"set","key":"/foo","prevValue":"bar","value":"bar","modifiedIndex":3}
```


### Authentication with HTTPS client certificates

We can also do authentication using CA certs.
The clients will provide their cert to the server and the server will check whether the cert is signed by the CA and decide whether to serve the request.

```sh
./etcd -f -name node0 -data-dir node0 -ca-file=./fixtures/ca/ca.crt -cert-file=./fixtures/ca/server.crt -key-file=./fixtures/ca/server.key.insecure
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
curl --key ./fixtures/ca/server2.key.insecure --cert ./fixtures/ca/server2.crt --cacert ./fixtures/ca/server-chain.pem -L https://127.0.0.1:4001/v1/keys/foo -XPUT -d value=bar -v
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
{"action":"set","key":"/foo","prevValue":"bar","value":"bar","modifiedIndex":3}
```


## Clustering

### Example cluster of three machines

Let's explore the use of etcd clustering.
We use Raft as the underlying distributed protocol which provides consistency and persistence of the data across all of the etcd instances.

Let start by creating 3 new etcd instances.

We use `-peer-addr` to specify server port and `-addr` to specify client port and `-data-dir` to specify the directory to store the log and info of the node in the cluster:

```sh
./etcd -peer-addr 127.0.0.1:7001 -addr 127.0.0.1:4001 -data-dir nodes/node1 -name node1
```

**Note:** If you want to run etcd on an external IP address and still have access locally, you'll need to add `-bind-addr 0.0.0.0` so that it will listen on both external and localhost addresses.
A similar argument `-peer-bind-addr` is used to setup the listening address for the server port.

Let's join two more nodes to this cluster using the `-peers` argument:

```sh
./etcd -peer-addr 127.0.0.1:7002 -addr 127.0.0.1:4002 -peers 127.0.0.1:7001 -data-dir nodes/node2 -name node2
./etcd -peer-addr 127.0.0.1:7003 -addr 127.0.0.1:4003 -peers 127.0.0.1:7001 -data-dir nodes/node3 -name node3
```

We can retrieve a list of machines in the cluster using the HTTP API:

```sh
curl -L http://127.0.0.1:4001/v1/machines
```

We should see there are three nodes in the cluster

```
http://127.0.0.1:4001, http://127.0.0.1:4002, http://127.0.0.1:4003
```

The machine list is also available via the main key API:

```sh
curl -L http://127.0.0.1:4001/v1/keys/_etcd/machines
```

```json
[{"action":"get","key":"/_etcd/machines/node1","value":"raft=http://127.0.0.1:7001\u0026etcd=http://127.0.0.1:4001","index":1},{"action":"get","key":"/_etcd/machines/node2","value":"raft=http://127.0.0.1:7002\u0026etcd=http://127.0.0.1:4002","index":1},{"action":"get","key":"/_etcd/machines/node3","value":"raft=http://127.0.0.1:7003\u0026etcd=http://127.0.0.1:4003","index":1}]
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
{"action":"set","key":"/foo","value":"bar","modifiedIndex":4}
```


### Killing Nodes in the Cluster

Now if we kill the leader of the cluster, we can get the value from one of the other two machines:

```sh
curl -L http://127.0.0.1:4002/v1/keys/foo
```

We can also see that a new leader has been elected:

```
curl -L http://127.0.0.1:4002/v1/leader
```

```
http://127.0.0.1:7002
```

or

```
http://127.0.0.1:7003
```


### Testing Persistence

Next we'll kill all the nodes to test persistence.
Type `CTRL-C` on each terminal and then rerun the same command you used to start each node.

Your request for the `foo` key will return the correct value:

```sh
curl -L http://127.0.0.1:4002/v1/keys/foo
```

```json
{"action":"get","key":"/foo","value":"bar","index":4}
```


### Using HTTPS between servers

In the previous example we showed how to use SSL client certs for client-to-server communication.
Etcd can also do internal server-to-server communication using SSL client certs.
To do this just change the `-*-file` flags to `-peer-*-file`.

If you are using SSL for server-to-server communication, you must use it on all instances of etcd.


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

**Erlang libraries**

- [marshall-lee/etcd.erl](https://github.com/marshall-lee/etcd.erl)

**Chef Integration**

- [coderanger/etcd-chef](https://github.com/coderanger/etcd-chef)

**Chef Cookbook**

- [spheromak/etcd-cookbook](https://github.com/spheromak/etcd-cookbook)

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
The result is that an 8 peer cluster can tolerate 3 peer failures and a 9 peer cluster can tolerate 4 nodes failures.
And in the best case when all 9 peers are responding the cluster will perform at the speed of the fastest 5 nodes.


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
openssl ca -config openssl.cnf -policy policy_anything -extensions ssl_client -out certs/node.crt -infiles node.csr
```


## Project Details

### Versioning

etcd uses [semantic versioning][semver].
New minor versions may add additional features to the API however.

You can get the version of etcd by issuing a request to /version:

```sh
curl -L http://127.0.0.1:4001/version
```

During the pre-v1.0.0 series of releases we may break the API as we fix bugs and get feedback.

[semver]: http://semver.org/


### License

etcd is under the Apache 2.0 license. See the [LICENSE][license] file for details.

[license]: https://github.com/coreos/etcd/blob/master/LICENSE
