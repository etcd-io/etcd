etcdctl
========

`etcdctl` is a command line client for [etcd][etcd].
It can be used in scripts or for administrators to explore an etcd cluster.

[etcd]: https://github.com/coreos/etcd


## Getting etcdctl

The latest release is available as a binary at [Github][github-release] along with etcd.

[github-release]: https://github.com/coreos/etcd/releases/

You can also build etcdctl from source using the build script found in the parent directory.

## Usage

### Setting Key Values

Set a value on the `/foo/bar` key:

```
$ etcdctl set /foo/bar "Hello world"
Hello world
```

Set a value on the `/foo/bar` key with a value that expires in 60 seconds:

```
$ etcdctl set /foo/bar "Hello world" --ttl 60
Hello world
```

Conditionally set a value on `/foo/bar` if the previous value was "Hello world":

```
$ etcdctl set /foo/bar "Goodbye world" --swap-with-value "Hello world"
Goodbye world
```

Conditionally set a value on `/foo/bar` if the previous etcd index was 12:

```
$ etcdctl set /foo/bar "Goodbye world" --swap-with-index 12
Goodbye world
```

Create a new key `/foo/bar`, only if the key did not previously exist:

```
$ etcdctl mk /foo/new_bar "Hello world"
Hello world
```

Create a new dir `/fooDir`, only if the key did not previously exist:

```
$ etcdctl mkdir /fooDir
```

Update an existing key `/foo/bar`, only if the key already existed:

```
$ etcdctl update /foo/bar "Hola mundo"
Hola mundo
```

Create or update a directory called `/mydir`:

```
$ etcdctl setDir /mydir
```


### Retrieving a key value

Get the current value for a single key in the local etcd node:

```
$ etcdctl get /foo/bar
Hello world
```

Get the current value for a key within the cluster:

```
$ etcdctl get /foo/bar --consistent
Hello world
```

Get the value of a key with additional metadata in a parseable format:

```
$ etcdctl -o extended get /foo/bar
Key: /foo/bar
Modified-Index: 72
TTL: 0
Etcd-Index: 72
Raft-Index: 5611
Raft-Term: 1

Hello World
```

### Listing a directory

Explore the keyspace using the `ls` command

```
$ etcdctl ls
/akey
/adir
$ etcdctl ls /adir
/adir/key1
/adir/key2
```

Add `--recursive` to recursively list subdirectories encountered.

```
$ etcdctl ls --recursive
/akey
/adir
/adir/key1
/adir/key2
```

Directories can also have a trailing `/` added to output using `-p`.

```
$ etcdctl ls -p
/akey
/adir/
```

### Deleting a key

Delete a key:

```
$ etcdctl rm /foo/bar
```

Delete an empty directory or a key-value pair

```
$ etcdctl rmdir /path/to/dir
```

or

```
$ etcdctl rm /path/to/dir --dir
```

Recursively delete a key and all child keys:

```
$ etcdctl rm /path/to/dir --recursive
```

Conditionally delete `/foo/bar` if the previous value was "Hello world":

```
$ etcdctl rm /foo/bar --with-value "Hello world"
```

Conditionally delete `/foo/bar` if the previous etcd index was 12:

```
$ etcdctl rm /foo/bar --with-index 12
```

### Watching for changes

Watch for only the next change on a key:

```
$ etcdctl watch /foo/bar
Hello world
```

Continuously watch a key:

```
$ etcdctl watch /foo/bar --forever
Hello world
.... client hangs forever until ctrl+C printing values as key change
```

Continuously watch a key, starting with a given etcd index:

```
$ etcdctl watch /foo/bar --forever --index 12
Hello world
.... client hangs forever until ctrl+C printing values as key change
```

Continuously watch a key and exec a program:

```
$ etcdctl exec-watch /foo/bar -- sh -c "env | grep ETCD"
ETCD_WATCH_ACTION=set
ETCD_VALUE=My configuration stuff
ETCD_MODIFIED_INDEX=1999
ETCD_KEY=/foo/bar
ETCD_WATCH_ACTION=set
ETCD_VALUE=My new configuration stuff
ETCD_MODIFIED_INDEX=2000
ETCD_KEY=/foo/bar
```

Continuously and recursively watch a key and exec a program:
```
$ etcdctl exec-watch --recursive /foo -- sh -c "env | grep ETCD"
ETCD_WATCH_ACTION=set
ETCD_VALUE=My configuration stuff
ETCD_MODIFIED_INDEX=1999
ETCD_KEY=/foo/bar
ETCD_WATCH_ACTION=set
ETCD_VALUE=My new configuration stuff
ETCD_MODIFIED_INDEX=2000
ETCD_KEY=/foo/barbar
```

## Return Codes

The following exit codes can be returned from etcdctl:

```
0    Success
1    Malformed etcdctl arguments
2    Failed to connect to host
3    Failed to auth (client cert rejected, ca validation failure, etc)
4    400 error from etcd
5    500 error from etcd
```

## Peers

If your etcd cluster isn't available on `http://127.0.0.1:4001` you can specify
a `--peers` flag or `ETCDCTL_PEERS` environment variable. You can list one peer,
or a comma-separated list of peers.

```
ETCDCTL_PEERS="http://10.0.28.1:4002" etcdctl set my-key to-a-value
ETCDCTL_PEERS="http://10.0.28.1:4002,http://10.0.28.2:4002,http://10.0.28.3:4002" etcdctl set my-key to-a-value
etcdctl --peers http://10.0.28.1:4002 my-key to-a-value
etcdctl --peers http://10.0.28.1:4002,http://10.0.28.2:4002,http://10.0.28.3:4002 etcdctl set my-key to-a-value
```

## Project Details

### Versioning

etcdctl uses [semantic versioning][semver].
Releases will follow lockstep with the etcd release cycle.

[semver]: http://semver.org/

### License

etcdctl is under the Apache 2.0 license. See the [LICENSE][license] file for details.

[license]: https://github.com/coreos/etcdctl/blob/master/LICENSE
