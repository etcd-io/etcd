# etcd
README version 0.1.0

[![Build Status](https://travis-ci.org/coreos/etcd.png)](https://travis-ci.org/coreos/etcd)

A highly-available key value store for shared configuration and service discovery. etcd is inspired by zookeeper and doozer, with a focus on:

* Simple: curl'able user facing API (HTTP+JSON)
* Secure: optional SSL client cert authentication
* Fast: benchmarked 1000s of writes/s per instance
* Reliable: Properly distributed using Raft

Etcd is written in Go and uses the [raft][raft] consensus algorithm to manage a highly-available replicated log.

See [etcdctl][etcdctl] for a simple command line client. Or feel free to just use curl, as in the examples below.

[raft]: https://github.com/coreos/go-raft
[etcdctl]: http://coreos.com/docs/etcdctl/

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

These examples will use a single node cluster to show you the basics of the etcd REST API. Lets start etcd:

```sh
./etcd -d node0 -n node0
```

This will bring up an etcd node listening on port 4001 for client communication and on port 7001 for server-to-server communication.
The `-d node0` argument tells etcd to write node configuration, logs and snapshots to the `./node0/` directory.
The `-n node0` tells the rest of the cluster that this node is named node0.

## Usage

### Setting the value to a key

Let’s set the first key-value pair to the node. In this case the key is `/message` and the value is `Hello world`.

```sh
curl -L http://127.0.0.1:4001/v1/keys/message -d value="Hello world"
```

```json
{"action":"SET","key":"/message","value":"Hello world","newKey":true,"index":3}
```

This response contains five fields. We will introduce three more fields as we try more commands.

1. The action of the request; we set the value via a POST request, thus the action is `SET`.

2. The key of the request; we set `/message` to `Hello world!`, so the key field is `/message`.
Notice we use a file system like structure to represent the key-value pairs. So each key starts with `/`.

3. The current value of the key; we set the value to`Hello world`.

4. If we set a new key; `/message` did not exist before, so this is a new key.

5. Index is the unique internal log index of the set request. Requests that change the log index include `SET`, `DELETE` and `TESTANDSET`. The `GET`, `LIST` and `WATCH` commands do not change state in the store and so they do not change the index. You may notice that in this example the index is 3, although it is the first request you sent to the server. This is because there are internal commands that also change the state like adding and syncing servers.

### Get the value of a key

Get the value that we just set in `/message` by issuing a GET:

```sh
curl -L http://127.0.0.1:4001/v1/keys/message
```

```json
{"action":"GET","key":"/message","value":"Hello world","index":3}
```
### Change the value of a key

Change the value of `/message` from `Hello world` to `Hello etcd` with another POST to the key:

```sh
curl -L http://127.0.0.1:4001/v1/keys/message -d value="Hello etcd"
```

```json
{"action":"SET","key":"/message","prevValue":"Hello world","value":"Hello etcd","index":4}
```

Notice that the `prevValue` is set to `Hello world`.

### Delete a key

Remove the `/message` key with a DELETE:

```sh
curl -L http://127.0.0.1:4001/v1/keys/message -X DELETE
```

```json
{"action":"DELETE","key":"/message","prevValue":"Hello etcd","index":5}
```

### Using key TTL

Keys in etcd can be set to expire after a specified number of seconds. That is done by setting a TTL (time to live) on the key when you POST:

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo -d value=bar -d ttl=5
```

```json
{"action":"SET","key":"/foo","value":"bar","newKey":true,"expiration":"2013-07-11T20:31:12.156146039-07:00","ttl":4,"index":6}
```

Note the last two new fields in response:

1. The expiration is the time that this key will expire and be deleted.

2. The ttl is the time to live of the key.

Now you can try to get the key by sending:

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo
```

If the TTL has expired, the key will be deleted, and you will be returned a 100.

```json
{"errorCode":100,"message":"Key Not Found","cause":"/foo"}
```

### Watching a prefix

We can watch a path prefix and get notifications if any key change under that prefix.

In one terminal, we send a watch request:

```sh
curl -L http://127.0.0.1:4001/v1/watch/foo
```

Now, we are watching at the path prefix `/foo` and wait for any changes under this path.

In another terminal, we set a key `/foo/foo` to `barbar` to see what will happen:

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo/foo -d value=barbar
```

The first terminal should get the notification and return with the same response as the set request.

```json
{"action":"SET","key":"/foo/foo","value":"barbar","newKey":true,"index":7}
```

However, the watch command can do more than this. Using the the index we can watch for commands that has happened in the past. This is useful for ensuring you don't miss events between watch commands.

Let's try to watch for the set command of index 6 again:

```sh
curl -L http://127.0.0.1:4001/v1/watch/foo -d index=6
```

The watch command returns immediately with the same response as previous.

### Atomic Test and Set

Etcd can be used as a centralized coordination service in a cluster and `TestAndSet` is the most basic operation to build distributed lock service. This command will set the value only if the client provided `prevValue` is equal the current key value.

Here is a simple example. Let's create a key-value pair first: `foo=one`.

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo -d value=one
```

Let's try an invalid `TestAndSet` command.
We can give another parameter prevValue to set command to make it a TestAndSet command.

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo -d prevValue=two -d value=three
```

This will try to test if the previous of the key is two, it is change it to three.

```json
{"errorCode":101,"message":"The given PrevValue is not equal to the value of the key","cause":"TestAndSet: one!=two"}
```

which means `testAndSet` failed.

Let us try a valid one.

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo -d prevValue=one -d value=two
```

The response should be

```json
{"action":"SET","key":"/foo","prevValue":"one","value":"two","index":10}
```

We successfully changed the value from “one” to “two”, since we give the correct previous value.

### Listing a directory

Last we provide a simple List command to list all the keys under a prefix path.

Let us create some keys first.

We already have `/foo/foo=barbar`

We create another one `/foo/foo_dir/foo=barbarbar`

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo/foo_dir/bar -d value=barbarbar
```

Now list the keys under `/foo`

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo/
```

We should see the response as an array of items

```json
[{"action":"GET","key":"/foo/foo","value":"barbar","index":10},{"action":"GET","key":"/foo/foo_dir","dir":true,"index":10}]
```

which meas `foo=barbar` is a key-value pair under `/foo` and `foo_dir` is a directory.

## Advanced Usage

### Transport security with HTTPS

Etcd supports SSL/TLS and client cert authentication for clients to server, as well as server to server communication

First, you need to have a CA cert `clientCA.crt` and signed key pair `client.crt`, `client.key`. This site has a good reference for how to generate self-signed key pairs:
http://www.g-loaded.eu/2005/11/10/be-your-own-ca/

For testing you can use the certificates in the `fixtures/ca` directory.

Next, lets configure etcd to use this keypair:

```sh
./etcd -n node0 -d node0 -clientCert=./fixtures/ca/server.crt -clientKey=./fixtures/ca/server.key.insecure -f
```

`-f` forces new node configuration if existing configuration is found (WARNING: data loss!)
`-clientCert` and `-clientKey` are the key and cert for transport layer security between client and server

You can now test the configuration using https:

```sh
curl --cacert fixtures/ca/ca.crt https://127.0.0.1:4001/v1/keys/foo -d value=bar -v
```

You should be able to see the handshake succeed.

```
...
SSLv3, TLS handshake, Finished (20):
...
```

And also the response from the etcd server.

```json
{"action":"SET","key":"/foo","value":"bar","newKey":true,"index":3}
```

### Authentication with HTTPS client certificates

We can also do authentication using CA certs. The clients will provide their cert to the server and the server will check whether the cert is signed by the CA and decide whether to serve the request.

```sh
./etcd -n node0 -d node0 -clientCAFile=./fixtures/ca/ca.crt -clientCert=./fixtures/ca/server.crt -clientKey=./fixtures/ca/server.key.insecure -f
```

```-clientCAFile``` is the path to the CA cert.

Try the same request to this server:

```sh
curl --cacert fixtures/ca/ca.crt https://127.0.0.1:4001/v1/keys/foo -d value=bar -v
```

The request should be rejected by the server.

```
...
routines:SSL3_READ_BYTES:sslv3 alert bad certificate
...
```

We need to give the CA signed cert to the server.

```sh
curl -L https://127.0.0.1:4001/v1/keys/foo -d value=bar -v --key myclient.key --cert myclient.crt -cacert clientCA.crt
```

You should able to see
```
...
SSLv3, TLS handshake, CERT verify (15):
...
TLS handshake, Finished (20)
```

And also the response from the server:

```json
{"action":"SET","key":"/foo","value":"bar","newKey":true,"index":3}
```

## Clustering

### Example cluster of three machines

Let's explore the use of etcd clustering. We use go-raft as the underlying distributed protocol which provides consistency and persistence of the data across all of the etcd instances.

Let start by creating 3 new etcd instances.

We use -s to specify server port and -c to specify client port and -d to specify the directory to store the log and info of the node in the cluster

```sh
./etcd -s 127.0.0.1:7001 -c 127.0.0.1:4001 -d nodes/node1 -n node1
```

**Note:** If you want to run etcd on external IP address and still have access locally you need to add `-cl 0.0.0.0` so that it will listen on both external and localhost addresses.
A similar argument `-sl` is used to setup the listening address for the server port.

Let the join two more nodes to this cluster using the -C argument:

```sh
./etcd -s 127.0.0.1:7002 -c 127.0.0.1:4002 -C 127.0.0.1:7001 -d nodes/node2 -n node2
./etcd -s 127.0.0.1:7003 -c 127.0.0.1:4003 -C 127.0.0.1:7001 -d nodes/node3 -n node3
```

Get the machines in the cluster:

```sh
curl -L http://127.0.0.1:4001/v1/machines
```

We should see there are three nodes in the cluster

```
http://127.0.0.1:4001, http://127.0.0.1:4002, http://127.0.0.1:4003
```

The machine list is also available via this API:

```sh
curl -L http://127.0.0.1:4001/v1/keys/_etcd/machines
```

```json
[{"action":"GET","key":"/_etcd/machines/node1","value":"raft=http://127.0.0.1:7001&etcd=http://127.0.0.1:4001","index":4},{"action":"GET","key":"/_etcd/machines/node2","value":"raft=http://127.0.0.1:7002&etcd=http://127.0.0.1:4002","index":4},{"action":"GET","key":"/_etcd/machines/node3","value":"raft=http://127.0.0.1:7003&etcd=http://127.0.0.1:4003","index":4}]
```

The key of the machine is based on the ```commit index``` when it was added. The value of the machine is ```hostname```, ```raft port``` and ```client port```.

Also try to get the current leader in the cluster

```
curl -L http://127.0.0.1:4001/v1/leader
```
The first server we set up should be the leader, if it has not died during these commands.

```
http://127.0.0.1:7001
```

Now we can do normal SET and GET operations on keys as we explored earlier.

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo -d value=bar
```

```json
{"action":"SET","key":"/foo","value":"bar","newKey":true,"index":5}
```

### Killing Nodes in the Cluster

Let's kill the leader of the cluster and get the value from the other machine:

```sh
curl -L http://127.0.0.1:4002/v1/keys/foo
```

A new leader should have been elected.

```
curl -L http://127.0.0.1:4001/v1/leader
```

```
http://127.0.0.1:7002
```

or

```
http://127.0.0.1:7003
```

You should be able to see this:

```json
{"action":"GET","key":"/foo","value":"bar","index":5}
```

It succeeded!

### Testing Persistence

OK. Next let us kill all the nodes to test persistence. And restart all the nodes use the same command as before.

Your request for the `foo` key will return the correct value:

```sh
curl -L http://127.0.0.1:4002/v1/keys/foo
```

```json
{"action":"GET","key":"/foo","value":"bar","index":5}
```

### Using HTTPS between servers

In the previous example we showed how to use SSL client certs for client to server communication. Etcd can also do internal server to server communication using SSL client certs. To do this just change the ```-client*``` flags to ```-server*```.

If you are using SSL for server to server communication, you must use it on all instances of etcd.

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
But, the command is not committed until the majority of the cluster machines receive that command.

Because of this majority voting property the ideal cluster should be kept small to keep speed up and be made up of an odd number of machines.

Odd numbers are good because if you have 8 machines the majority will be 5 and if you have 9 machines the majority with be 5.
The result is that an 8 machine cluster can tolerate 3 machine failures and a 9 machine cluster can tolerate 4 nodes failures.
And in the best case when all 9 machines are responding the cluster will perform at the speed of the fastest 5 nodes.

### Why SSLv3 alert handshake failure when using SSL client auth?
The `TLS` pacakge of `golang` checks the key usage of certificate public key before using it. To use the certificate public key to do client auth, we need to add `clientAuth` to `Extended Key Usage` when creating the certificate public key.

Here is how to do it:

Add the following section to your openssl.cnf:

```
[ ssl_client ]                                                                                                                                            
...
  extendedKeyUsage = clientAuth
...
```

When creating the cert be sure to reference it in the -extensions flag:

```
openssl ca -config openssl.cnf -policy policy_anything -extensions ssl_client -out certs/node.crt -infiles node.csr
```

## Project Details

### Versioning

etcd uses [semantic versioning][semver].
When we release v1.0.0 of etcd we will promise not to break the "v1" REST API.
New minor versions may add additional features to the API however.

You can get the version of etcd by issuing a request to /version:

```sh
curl -L http://127.0.0.1:4001/version
```

During the v0 series of releases we may break the API as we fix bugs and get feedback.

[semver]: http://semver.org/

### License

etcd is under the Apache 2.0 license. See the [LICENSE][license] file for details.

[license]: https://github.com/coreos/etcd/blob/master/LICENSE
