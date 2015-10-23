## Usage

Benchmark 3-member etcd cluster to get its read and write performance.

## Instructions

1. Start 3-member etcd cluster on 3 machines
2. Update `$leader` and `$servers` in the script
3. Apply [connection reuse patch](#connection-reuse-patch-for-boom) to [this boom revision](https://github.com/rakyll/boom/commit/79153762c259a71f2febd651a619c8b20d0f5178) and build boom
3. Run the script in a separate machine

## Caveat

1. Set environment variable `GOMAXPROCS` as the number of available cores to maximize CPU resources for both etcd member and bench process.
2. Set the number of open files per process as 10000 for amounts of client connections for both etcd member and benchmark process.

## Connection Reuse Patch for boom

```
diff --git a/boomer/run.go b/boomer/run.go
index 0ed129e..062f386 100644
--- a/boomer/run.go
+++ b/boomer/run.go
@@ -16,6 +16,7 @@ package boomer

 import (
 	"crypto/tls"
+	"io/ioutil"

 	"sync"

@@ -65,6 +66,7 @@ func (b *Boomer) worker(wg *sync.WaitGroup, ch chan *http.Request) {
 		if err == nil {
 			size = resp.ContentLength
 			code = resp.StatusCode
+			ioutil.ReadAll(resp.Body)
 			resp.Body.Close()
 		}
 		if b.bar != nil {
```

This patch changes `boom` to read the response body until it is complete, so that the underlying transport can reuse the connection immediately. This avoids the `dial tcp 127.0.0.1:4001: connect: can't assign requested address` error when benchmarking etcd.
