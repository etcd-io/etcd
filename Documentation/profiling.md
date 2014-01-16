## Profiling

etcd exposes profiling information from the Go pprof package over HTTP.
The basic browseable interface can be found at `http://127.0.0.1:4001/debug/pprof`.

**NOTE**: In the following examples you need to ensure that the `./bin/etcd` is
identical to the `./bin/etcd` that you are targetting (same git hash, arch,
platform, etc).

### Heap memory profile

```
go tool pprof ./bin/etcd http://127.0.0.1:4001/debug/pprof/heap
```

### CPU profile

```
go tool pprof ./bin/etcd http://127.0.0.1:4001/debug/pprof/profile
```

### Blocked goroutine profile

```
go tool pprof ./bin/etcd http://127.0.0.1:4001/debug/pprof/block
```

For more information on using the tools see http://blog.golang.org/profiling-go-programs
