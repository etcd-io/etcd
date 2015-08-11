# etcd/client

[![GoDoc](https://godoc.org/github.com/coreos/etcd/client?status.png)](https://godoc.org/github.com/coreos/etcd/client)

## Install

```bash
go get github.com/coreos/etcd/client
```

## Usage

```go
package main

import (
	"log"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

func main() {
	cfg := client.Config{
		Endpoints:               []string{"http://127.0.0.1:2379"},
		Transport:               client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}
	c, err := client.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	kapi := client.NewKeysAPI(c)
	resp, err := kapi.Set(context.Background(), "foo", "bar", nil)
	if err != nil {
		log.Fatal(err)
	}
}
```

## Error Handling

etcd client might return three types of errors.

- context error

Each API call has its first parameter as `context`. A context can be canceled or attached a deadline with. If the context is canceled or reaches its deadline, the responding context error will be returned no matter what internal errors the API call has already encountered.

- cluster error

Each API call tries to send request to the cluster endpoints one by one until it successfully gets a response. If a requests to an endpoints fails due to exceeding per request timeout or connection issues, the error will be added into a list of errors. If all possible endpoints fail, a cluster error that includes all encountered errors will be returned.

- response error

If the response gets from the cluster is invalid, a plain string error will be returned. For example, it might be a invalid JSON error.

Here is the example code to handle client errors:

```go
cfg := client.Config{Endpoints: []string{"http://etcd1:2379,http://etcd2:2379,http://etcd3:2379"}}
c, err := client.New(cfg)
if err != nil {
	log.Fatal(err)
}

kapi := client.NewKeysAPI(c)
resp, err := kapi.Set(ctx, "test", "bar", nil)
if err != nil {
	if err == context.Canceled {
		// ctx is canceled by another routine
	} else if err == context.DeadlineExceeded {
		// ctx is attached with a deadline and it exceeded
	} else if cerr, ok := err.(ClusterError); ok {
		// process (cerr.Errors)
	} else {
		// bad cluster endpoints, which are not etcd servers
	}
}
```


## Caveat

1. etcd/client always talks to the same endpoint if it works well. This saves socket resources, and improves efficiency for both client and server side. It doesn't hurt the consistent view of the client because each etcd member has data replication.

2. etcd/client does round-robin rotation on other available endpoints when it fails to talk to the endpoint in use. For example, if the member that etcd/client connects to is hard killed, etcd/client will fail on the first attempt with the killed member, and succeed on the second attempt with another member. If it fails to talk to all available endpoints, it will return the last error happened.

3. Default etcd/client cannot handle the case that the remote server is SIGSTOPed now. TCP keepalive mechanism doesn't help in this scenario because operating system may still send TCP keep-alive packets. We will improve it, but it is not in high priority because we don't see a solid real-life case which server is stopped but connection is alive.

4. etcd/client cannot detect whether the member in use is healthy when doing read requests. If the member is isolated from the cluster, etcd/client may retrieve outdated data. We will improve this.
