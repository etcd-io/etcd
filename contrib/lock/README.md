# What is this?
This directory provides an executable example of the scenarios described in [the article by Martin Kleppmann][fencing].

Generally speaking, a lease-based lock service cannot provide mutual exclusion to processes. This is because such a lease mechanism depends on the physical clock of both the lock service and client processes. Many factors (e.g. stop-the-world GC pause of a language runtime) can cause false expiration of a granted lease as depicted in the below figure: ![unsafe lock][unsafe-lock]

As discussed in [notes on the usage of lock and lease][why], such a problem can be solved with a technique called version number validation or fencing tokens. With this technique a shared resource (storage in the figures) needs to validate requests from clients based on their tokens like this: ![fencing tokens][fencing-tokens]

This directory contains two programs: `client` and `storage`. With `etcd`, you can reproduce the expired lease problem of distributed locking and a simple example solution of the validation technique which can avoid incorrect access from a client with an expired lease.

`storage` works as a very simple key value in-memory store which is accessible through HTTP and a custom JSON protocol. `client` works as client processes which tries to write a key/value to `storage` with coordination of etcd locking.

## How to build

For building `client` and `storage`, just execute `go build` in each directory.

## How to try

At first, you need to start an etcd cluster, which works as lock service in the figures. On top of the etcd source directory, execute commands like below:
```
$ make      # build etcd
$ bin/etcd  # start etcd
```

Then run `storage` command in `storage` directory:
```
$ ./storage
```

Now client processes ("Client 1" and "Client 2" in the figures) can be started. At first, execute below command for starting a client process which corresponds to "Client 1":
```
$ ./client 1
```
It will show an output like this:
```
client 1 starts
created etcd client and session
acquired lock, version: 694d82254d5fa305
please manually revoke the lease using 'etcdctl lease revoke 694d82254d5fa305' or wait for it to expire, then start executing client 2 and hit any key...
```

Verify the lease was created using:
```
$ bin/etcdctl lease list
found 1 leases
694d82254d5fa305
```

Then proceed to manually revoke the lease using:
```
$ bin/etcdctl lease revoke 694d82254d5fa305
lease 694d82254d5fa305 revoked
```

Now another client process can be started like this:
```
$ ./client 2
client 2 starts
created etcd client and session
acquired lock, version: 694d82254e18770a
this is client 2, continuing
```
If things go well the second client process invoked as `./client 2` finishes soon. It successfully writes a key to `storage` process. 

After checking this, please hit any key for `./client 1` and resume the process. It will show an output like below:
```
resuming client 1
expected fail to write to storage with old lease version: error: given version (694d82254d5fa305) is different from the existing version (694d82254e18770a)
```

[fencing]: https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html
[fencing-tokens]: https://martin.kleppmann.com/2016/02/fencing-tokens.png
[unsafe-lock]: https://martin.kleppmann.com/2016/02/unsafe-lock.png
[why]: https://etcd.io/docs/next/learning/why/#notes-on-the-usage-of-lock-and-lease
