# etcd

[![Build Status](https://travis-ci.org/coreos/etcd.png)](https://travis-ci.org/coreos/etcd)

A highly-available key value store for shared configuration and service discovery. etcd is inspired by zookeeper and doozer, with a focus on:

* Simple: curl'able user facing API (HTTP+JSON)
* Secure: optional SSL client cert authentication
* Fast: benchmarked 1000s of writes/s per instance
* Reliable: Properly distributed using Raft

Etcd is written in go and uses the [raft][raft] consensus algorithm to manage a highly availably replicated log. 

See [go-etcd][go-etcd] for a native go client. Or feel free to just use curl, as in the examples below. 

[raft]: https://github.com/coreos/go-raft
[go-etcd]: https://github.com/coreos/go-etcd

## Getting Started

### Building

To build etcd run the build script. This will generate a binary in the base directory called `./etcd`.

```
./build
```

### Running a single node

These examples will use a single node cluster to show you the basics of the etcd REST API. Lets start etcd:

```sh
./etcd
```

This will bring up a node, which will be listening on internal port 7001 (for server communication) and external port 4001 (for client communication)

#### Setting the value to a key

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

5. Index field is the unique request index of the set request. Each sensitive request we send to the server will have a unique request index. The current sensitive request are `SET`, `DELETE` and `TESTANDSET`. All of these request will change the state of the key-value store system, thus they are sensitive. `GET`, `LIST` and `WATCH` are non-sensitive commands. Those commands will not change the state of the key-value store system. You may notice that in this example the index is 3, although it is the first request you sent to the server. This is because there are some internal commands that also change the state of the server, we also need to assign them command indexes(Command used to add a server and sync the servers).

#### Getting the value of a key

Get the value that we just set in `/message` by issuing a GET:

```sh
curl -L http://127.0.0.1:4001/v1/keys/message
```

```json
{"action":"GET","key":"/message","value":"Hello world","index":3}
```
#### Changing the value of a key

Change the value of `/message` from `Hello world` to `Hello etcd` with another POST to the key:

```sh
curl -L http://127.0.0.1:4001/v1/keys/message -d value="Hello etcd"
```

```json
{"action":"SET","key":"/message","prevValue":"Hello world","value":"Hello etcd","index":4}
```

Notice that the `prevValue` is set to `Hello world`.

#### Deleting a key

Remove the `/message` key with a DELETE:

```sh
curl -L http://127.0.0.1:4001/v1/keys/message -X DELETE
```

```json
{"action":"DELETE","key":"/message","prevValue":"Hello etcd","index":5}
```

#### Using a TTL on a key

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

If the TTL has expired, the key will be deleted, and you will be returned a 404.

```html
404 page not found
```


#### Watching a prefix

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
curl -L http://127.0.0.1:4001/v1/watch/foo -d index=7
```

The watch command returns immediately with the same response as previous.

#### Atomic Test and Set

Etcd servers will process all the command in sequence atomically. Thus it can be used as a centralized coordination service in a cluster.

`TestAndSet` is the most basic operation to build distributed lock service.

The basic logic is to test whether the given previous value is equal to the value of the key, if equal etcd will change the value of the key to the given value.

Here is a simple example. Let's create a key-value pair first: `testAndSet=one`.

```sh
curl -L http://127.0.0.1:4001/v1/keys/testAndSet -d value=one
```

Let's try an invaild `TestAndSet` command.
We can give another parameter prevValue to set command to make it a TestAndSet command.

```sh
curl -L http://127.0.0.1:4001/v1/keys/testAndSet -d prevValue=two -d value=three
```

This will try to test if the previous of the key is two, it is change it to three.

```json
{"errorCode":101,"message":"The given PrevValue is not equal to the value of the key","cause":"TestAndSet: one!=two"}
```

which means `testAndSet` failed.

Let us try a vaild one.

```sh
curl -L http://127.0.0.1:4001/v1/keys/testAndSet -d prevValue=one -d value=two
```

The response should be

```json
{"action":"SET","key":"/testAndSet","prevValue":"one","value":"two","index":10}
```

We successfully changed the value from “one” to “two”, since we give the correct previous value.


#### Listing directory

Last we provide a simple List command to list all the keys under a prefix path.

Let us create some keys first.

We already have `/foo/foo=barbar`

We create another one `/foo/foo_dir/foo=barbarbar`

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo/foo_dir/bar -d value=barbarbar
```

Let us list them next.

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo/
```

We should see the response as an array of items

```json
[{"action":"GET","key":"/foo/foo","value":"barbar","index":10},{"action":"GET","key":"/foo/foo_dir","dir":true,"index":10}]
```

which meas `foo=barbar` is a key-value pair under `/foo` and `foo_dir` is a directory.

#### Using HTTPS between server and client
Etcd supports SSL/TLS and client cert authentication for clients to server, as well as server to server communication

Before that we need to have a CA cert`clientCA.crt` and signed key pair `client.crt`, `client.key` .

This site has a good reference for how to generate self-signed key pairs
```url
http://www.g-loaded.eu/2005/11/10/be-your-own-ca/
```

```sh
./etcd -clientCert client.crt -clientKey client.key -f
```

`-f` forces new node configuration if existing configuration is found (WARNING: data loss!)
`-clientCert` and `-clientKey` are the key and cert for transport layer security between client and server

```sh
curl -L https://127.0.0.1:4001/v1/keys/foo -d value=bar -v -k
```

or 

```sh
curl -L https://127.0.0.1:4001/v1/keys/foo -d value=bar -v -cacert clientCA.crt
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

We also can do authentication using CA cert. The clients will also need to provide their cert to the server. The server will check whether the cert is signed by the CA and decide whether to serve the request.

```sh
./etcd -clientCert client.crt -clientKey client.key -clientCAFile clientCA.crt -f
```

```-clientCAFile``` is the path to the CA cert.

Try the same request to this server.
```sh
curl -L https://127.0.0.1:4001/v1/keys/foo -d value=bar -v -k
```
or 

```sh
curl -L https://127.0.0.1:4001/v1/keys/foo -d value=bar -v -cacert clientCA.crt
```

The request should be rejected by the server.
```
...
routines:SSL3_READ_BYTES:sslv3 alert bad certificate
...
```

We need to give the CA signed cert to the server. 
```sh
curl -L https://127.0.0.1:4001/v1/keys/foo -d value=bar -v --key myclient.key --cert myclient.crt -k
```

or

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

And also the response from the server
```json
{"action":"SET","key":"/foo","value":"bar","newKey":true,"index":3}
```

### Setting up a cluster of three machines

Next let's explore the use of etcd clustering. We use go-raft as the underlying distributed protocol which provides consistency and persistence of the data across all of the etcd instances.

Let start by creating 3 new etcd instances.

We use -s to specify server port and -c to specify client port and -d to specify the directory to store the log and info of the node in the cluster

```sh
./etcd -s 7001 -c 4001 -d nodes/node1
```

Let the join two more nodes to this cluster using the -C argument:

```sh
./etcd -c 4002 -s 7002 -C 127.0.0.1:7001 -d nodes/node2
./etcd -c 4003 -s 7003 -C 127.0.0.1:7001 -d nodes/node3
```

Get the machines in the cluster

```sh
curl -L http://127.0.0.1:4001/machines
```

We should see there are three nodes in the cluster

```
0.0.0.0:4001,0.0.0.0:4002,0.0.0.0:4003
```

Machine list is also available via this API

```sh 
curl -L http://127.0.0.1:4001/v1/keys/_etcd/machines
```

```json
[{"action":"GET","key":"/machines/node1","value":"0.0.0.0,7001,4001","index":4},{"action":"GET","key":"/machines/node3","value":"0.0.0.0,7002,4002","index":4},{"action":"GET","key":"/machines/node4","value":"0.0.0.0,7003,4003","index":4}]
```

The key of the machine is based on the ```commit index``` when it was added. The value of the machine is ```hostname```, ```raft port``` and ```client port```.

Also try to get the current leader in the cluster

```
curl -L http://127.0.0.1:4001/leader
```
The first server we set up should be the leader, if it has not dead during these commands.

```
0.0.0.0:7001
```

Now we can do normal SET and GET operations on keys as we explored earlier.

```sh
curl -L http://127.0.0.1:4001/v1/keys/foo -d value=bar
```

When the client sends a sensitive command (`set`, `delete`, `testAndset` ) to the server, the command needs to be redirect to the leader of the cluster.

So we add the ` -L ` flag to make curl follow location hints in http location header when there is a redirection http response.

The response should be 
```json
{"action":"SET","key":"/foo","value":"bar","newKey":true,"index":5}
```

#### Killing Nodes in the Cluster

Let's kill the leader of the cluster and get the value from the other machine:

```sh
curl -L http://127.0.0.1:4002/v1/keys/foo
```

A new leader should have been elected.

```
curl -L http://127.0.0.1:4001/leader
```

```
0.0.0.0:7002 or 0.0.0.0:7003
```

You should be able to see this:

```json
{"action":"GET","key":"/foo","value":"bar","index":5}
```

It succeed!

#### Testing Persistence

OK. Next let us kill all the nodes to test persistence. And restart all the nodes use the same command as before.

Your request for the `foo` key will return the correct value:

```sh
curl -L http://127.0.0.1:4002/v1/keys/foo
```

```json
{"action":"GET","key":"/foo","value":"bar","index":5}
```

#### Using HTTPS between servers
In the previous example we showed how to use SSL client certs for client to server communication. Etcd can also do internal server to server communication using SSL client certs. To do this just change the ```-client*``` flags to ```-server*```.
If you are using SSL for server to server communication, you must use it on all instances of etcd.

