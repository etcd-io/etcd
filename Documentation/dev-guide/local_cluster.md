---
title: Set up a local cluster
weight: 3125
description: Configuring local clusters for testing and development 
---

For testing and development deployments, the quickest and easiest way is to configure a local cluster. For a production deployment, refer to the [clustering][clustering] section.

## Local standalone cluster

### Starting a cluster

Run the following to deploy an etcd cluster as a standalone cluster:

```
$ ./etcd
...
```

If the `etcd` binary is not present in the current working directory, it might be located either at `$GOPATH/bin/etcd` or at `/usr/local/bin/etcd`. Run the command appropriately.

The running etcd member listens on `localhost:2379` for client requests.

### Interacting with the cluster

Use `etcdctl` to interact with the running cluster:

1. Store an example key-value pair in the cluster:

    ```
      $ ./etcdctl put foo bar
      OK
    ```

    If OK is printed, storing key-value pair is successful.

2. Retrieve the value of `foo`:

    ```
    $ ./etcdctl get foo
    bar
    ```

    If `bar` is returned, interaction with the etcd cluster is working as expected.

## Local multi-member cluster

### Starting a cluster

A `Procfile` at the base of the etcd git repository is provided to easily configure a local multi-member cluster. To start a multi-member cluster, navigate to the root of the etcd source tree and perform the following:

1. Install `goreman` to control Procfile-based applications:

    ```
    $ go get github.com/mattn/goreman
    ```

2. Start a cluster with `goreman` using etcd's stock Procfile:

    ```
    $ goreman -f Procfile start
    ```

    The members start running. They listen on `localhost:2379`, `localhost:22379`, and `localhost:32379` respectively for client requests.

### Interacting with the cluster

Use `etcdctl` to interact with the running cluster:

1. Print the list of members:

    ```
    $ etcdctl --write-out=table --endpoints=localhost:2379 member list
    ```
    The list of etcd members are displayed as follows:

    ```
    +------------------+---------+--------+------------------------+------------------------+
    |        ID        | STATUS  |  NAME  |       PEER ADDRS       |      CLIENT ADDRS      |
    +------------------+---------+--------+------------------------+------------------------+
    | 8211f1d0f64f3269 | started | infra1 | http://127.0.0.1:2380  | http://127.0.0.1:2379  |
    | 91bc3c398fb3c146 | started | infra2 | http://127.0.0.1:22380 | http://127.0.0.1:22379 |
    | fd422379fda50e48 | started | infra3 | http://127.0.0.1:32380 | http://127.0.0.1:32379 |
    +------------------+---------+--------+------------------------+------------------------+
    ```

2. Store an example key-value pair in the cluster:

    ```
    $ etcdctl put foo bar
    OK
    ```

    If OK is printed, storing key-value pair is successful.

### Testing fault tolerance  

To exercise etcd's fault tolerance, kill a member and attempt to retrieve the key.

1. Identify the process name of the member to be stopped.

    The `Procfile` lists the properties of the multi-member cluster. For example, consider the member with the process name, `etcd2`.

2. Stop the member:

    ```
    # kill etcd2
    $ goreman run stop etcd2
    ```

3. Store a key:

    ```
    $ etcdctl put key hello
    OK
    ```

4.  Retrieve the key that is stored in the previous step:

    ```
    $ etcdctl get key
    hello
    ```

5. Retrieve a key from the stopped member:

    ```
    $ etcdctl --endpoints=localhost:22379 get key
    ```

    The command should display an error caused by connection failure:

    ```
    2017/06/18 23:07:35 grpc: Conn.resetTransport failed to create client transport: connection error: desc = "transport: dial tcp 127.0.0.1:22379: getsockopt: connection refused"; Reconnecting to "localhost:22379"
    Error:  grpc: timed out trying to connect
    ```
6. Restart the stopped member:

    ```
    $ goreman run restart etcd2
    ```

7. Get the key from the restarted member:

    ```
    $ etcdctl --endpoints=localhost:22379 get key
    hello
    ```

    Restarting the member re-establish the connection. `etcdctl` will now be able to retrieve the key successfully. To learn more about interacting with etcd, read [interacting with etcd section][interacting].

[interacting]: ./interacting_v3.md
[clustering]: ../op-guide/clustering.md
