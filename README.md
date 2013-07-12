etcd
====
## Overview

## Start with examples

### Setting up a node

```sh
./etcd 
```

This will bring up a node, which will be listening on internal port 7001 (for server communication) and external port 4001 (for client communication)

#### Setting the value to a key

Let’s set the first key-value pair to the node. In this case the key is `/message` and the value is `Hello world`.

```sh
curl http://127.0.0.1:4001/v1/keys/message -d value="Hello world"
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

5. Index field is the unique request index of the set request. Each sensitive request we send to the server will have a unique request index. The current sensitive request are `SET`, `DELETE` and `TESTANDSET`. All of these request will change the state of the key-value store system, thus they are sensitive. `GET`, `LIST` and `WATCH` are non-sensitive commands. Those commands will not change the state of the key-value store system. 
You may notice that in this example the index is 3, although it is the first request you sent to the server. This is because there are some internal commands that also change the state of the server, we also need to assign them command indexes(Command used to add a server and sync the servers).

#### Getting the value of a key

```sh
curl http://127.0.0.1:4001/v1/keys/message 
```

You should receive the response as 
```json
{"action":"GET","key":"/message","value":"Hello world","index":3}
```
#### Changing the value of a key

We change the value of `/message` from `Hello world` to `Hello etcd`

```sh
curl http://127.0.0.1:4001/v1/keys/message -d value="Hello etcd"
```

```json
{"action":"SET","key":"/message","prevValue":"Hello world","value":"Hello etcd","index":4}
```

There is a new field in the response: prevValue. It is the value of the key before the change happened. 

#### Deleting a key

```sh
curl http://127.0.0.1:4001/v1/keys/message -X DELETE
```

You should see the response as 

```json
{"action":"DELETE","key":"/message","prevValue":"Hello etcd","index":5}
```
 
#### Using time to live key

```sh
curl http://127.0.0.1:4001/v1/keys/foo -d value=bar -d ttl=5
```

You should see the similar response as (not exact same, they should have different expiration time)

```json
{"action":"SET","key":"/foo","value":"bar","newKey":true,"expiration":"2013-07-11T20:31:12.156146039-07:00","ttl":4,"index":6}
```

There are the last two new fields in response.

Expiration field is the time that this key will expire and be deleted. 

Ttl field is the time to live of the key, it can be derived from current time and expiration time.

Now you can try to get the key by sending

```sh
curl http://127.0.0.1:4001/v1/keys/foo 
```
You can expect the ttl is counting down and after 5 seconds you should see this,

```html
404 page not found
```

which indicates the key has expired and was deleted. 

#### Watching a prefix

Watch command can watch as a prefix path and get notification if any key changes after the prefix.

In one terminal, we send a watch request:
```sh
curl http://127.0.0.1:4001/v1/watch/foo
```

Now, we are watching at the path prefix `/foo` and wait for any changes under this path.

In another terminal, we set a key `/foo/foo` to `barbar` to see what will happen:

```sh
curl http://127.0.0.1:4001/v1/keys/foo/foo -d value=barbar
```

The first terminal should get the notification and return with the same response as the set request. 

```json
{"action":"SET","key":"/foo/foo","value":"barbar","newKey":true,"index":7}
```

OK. Watch command can do more than this. We have index and in etcd we store the most recent 1000 responses by default, which allow us to watch for previous commands.

Let us try to watch for the set command of index 6 again.

```sh
curl http://127.0.0.1:4001/v1/watch/foo -d index=7
```

You should see the watch command return immediately with the same response as previous.

#### Trying TestAndSet

Etcd servers will process all the command in sequence atomically, thus it can be used as a centralized decision making cluster.

TestAndSet is the most basic operation to build distributed lock service and more interesting stuff.

What it does is to test whether the given previous value is equal to the value of the key, if equal etcd will change the value of the key to the given value.

Here is a simple example. 
Let us create a key-value pair first: `testAndSet=one`.
```sh
curl http://127.0.0.1:4001/v1/keys/testAndSet -d value=one
```

Let us try a invaild `TestAndSet` command.
```sh
curl http://127.0.0.1:4001/v1/testAndSet/testAndSet -d prevValue=two -d value=three
```

This will try to test if the previous of the key is two, it is change it to three.

The response should be 
```html
Test one==two fails
```

which means `testAndSet` failed. 

Let us try a vaild one.

```sh
curl http://127.0.0.1:4001/v1/testAndSet/testAndSet -d prevValue=one -d value=two
```

The response should be 

```json
{"action":"SET","key":"/testAndSet","prevValue":"one","value":"two","index":10}
```

We successfully change the value from “one” to “two”, since we give the correct previous value.


#### Listing directory
Last we provide a simple List command to list all the keys under a prefix path.

Let us create some keys first.

We already have `/foo/foo=barbar`

We create another one `/foo/foo_dir/foo=barbarbar`

```sh
http://127.0.0.1:4001/v1/keys/foo/foo_dir/bar -d value=barbarbar
```

Let us list them next.

```sh
curl http://127.0.0.1:4001/v1/list/foo/
```

We should see the response as 

```json
{"Key":"foo","Value":"barbar","Type":"f"} {"Key":"foo_dir","Value":".","Type":"d"}
```

which meas `foo=barbar` is a key-value pair under `/foo` and `foo_dir` is a directory.

### Setting up a cluster of three machines
Next we can explore the power of etcd cluster. We use go-raft as the underlay distributed protocol which provide consistency and persistence of all the machines in the cluster. The will allow if the minor machine dies, the cluster will still be able to performance correctly. Also if most of the machines dead and restart,  we will recover from the previous state of the cluster.

Let us create 3 new machines.

The first one will be 

We use -s to specify server port and -c to specify client port and -d to specify the directory to store the log and info of the node in the cluster

```sh
./etcd -s 7001 -c 4001 -d nodes/node1
```

We use -C to specify the Cluster

Let the second one join it.
```sh
./etcd -c 4002 -s 7002 -C 127.0.0.1:7001 -d nod/node2
```

And the third one:
```sh
./etcd -c 4003 -s 7003 -C 127.0.0.1:7001 -d nod/node3
```

Let us add a key to the cluster of 3 nodes.

```sh
curl http://127.0.0.1:4001/v1/keys/foo -d value=bar
```

```json
{"action":"SET","key":"/foo","value":"bar","newKey":true,"index":5}
```

Let us kill the leader of the cluster to see what will happen.

Kill the first node which is the current leader

Try to get the value from the other machine

```sh
curl http://127.0.0.1:4002/v1/keys/foo
```

You should be able to see this

```json
{"action":"GET","key":"/foo","value":"bar","index":5}
```

It succeed!

OK. Next let us kill all the nodes to test persistence. And restart all the nodes use the same command before.

Try 
```sh
curl http://127.0.0.1:4002/v1/keys/foo
```
You should able to see 
```json
{"action":"GET","key":"/foo","value":"bar","index":5}
```
