## Modules

etcd has a number of modules that are built on top of the core etcd API.
These modules provide things like dashboards, locks and leader election.

### Dashboard

An HTML dashboard can be found at `http://127.0.0.1:4001/mod/dashboard/`.
This dashboard is compiled into the etcd binary and uses the same API as regular etcd clients.

Use the `-cors='*'` flag to allow your browser to request information from the current master as it changes.

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



