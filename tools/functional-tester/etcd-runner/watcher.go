// Copyright 2016 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/stringutil"
	"golang.org/x/time/rate"
)

func runWatcher(getClient getClientFunc, limit int) {
	ctx := context.Background()
	for round := 0; round < limit; round++ {
		performWatchOnPrefixes(ctx, getClient, round)
	}
}

func performWatchOnPrefixes(ctx context.Context, getClient getClientFunc, round int) {
	runningTime := 60 * time.Second // time for which operation should be performed
	noOfPrefixes := 36              // total number of prefixes which will be watched upon
	watchPerPrefix := 10            // number of watchers per prefix
	reqRate := 30                   // put request per second
	keyPrePrefix := 30              // max number of keyPrePrefixs for put operation

	prefixes := stringutil.UniqueStrings(5, noOfPrefixes)
	keys := stringutil.RandomStrings(10, keyPrePrefix)

	roundPrefix := fmt.Sprintf("%16x", round)

	var (
		revision int64
		wg       sync.WaitGroup
		gr       *clientv3.GetResponse
		err      error
	)

	client := getClient()
	defer client.Close()

	// get revision using get request
	gr = getWithRetry(client, ctx, "non-existent")
	revision = gr.Header.Revision

	ctxt, cancel := context.WithDeadline(ctx, time.Now().Add(runningTime))
	defer cancel()

	// generate and put keys in cluster
	limiter := rate.NewLimiter(rate.Limit(reqRate), reqRate)

	go func() {
		var modrevision int64
		for _, key := range keys {
			for _, prefix := range prefixes {
				key := roundPrefix + "-" + prefix + "-" + key

				// limit key put as per reqRate
				if err = limiter.Wait(ctxt); err != nil {
					break
				}

				modrevision = 0
				gr = getWithRetry(client, ctxt, key)
				kvs := gr.Kvs
				if len(kvs) > 0 {
					modrevision = gr.Kvs[0].ModRevision
				}

				for {
					txn := client.Txn(ctxt)
					_, err = txn.If(clientv3.Compare(clientv3.ModRevision(key), "=", modrevision)).Then(clientv3.OpPut(key, key)).Commit()

					if err == nil {
						break
					}

					if err == context.DeadlineExceeded {
						return
					}
				}
			}
		}
	}()

	ctxc, cancelc := context.WithCancel(ctx)

	wcs := make([]clientv3.WatchChan, 0)
	rcs := make([]*clientv3.Client, 0)

	wg.Add(noOfPrefixes * watchPerPrefix)
	for _, prefix := range prefixes {
		for j := 0; j < watchPerPrefix; j++ {
			go func(prefix string) {
				defer wg.Done()

				rc := getClient()
				rcs = append(rcs, rc)

				wc := rc.Watch(ctxc, prefix, clientv3.WithPrefix(), clientv3.WithRev(revision))
				wcs = append(wcs, wc)
				for n := 0; n < len(keys); {
					select {
					case watchChan := <-wc:
						for _, event := range watchChan.Events {
							expectedKey := prefix + "-" + keys[n]
							receivedKey := string(event.Kv.Key)
							if expectedKey != receivedKey {
								log.Fatalf("expected key %q, got %q for prefix : %q\n", expectedKey, receivedKey, prefix)
							}
							n++
						}
					case <-ctxt.Done():
						return
					}
				}
			}(roundPrefix + "-" + prefix)
		}
	}
	wg.Wait()

	// cancel all watch channels
	cancelc()

	// verify all watch channels are closed
	for e, wc := range wcs {
		if _, ok := <-wc; ok {
			log.Fatalf("expected wc to be closed, but received %v", e)
		}
	}

	for _, rc := range rcs {
		rc.Close()
	}

	deletePrefixWithRety(client, ctx, roundPrefix)
}

func deletePrefixWithRety(client *clientv3.Client, ctx context.Context, key string) {
	for {
		if _, err := client.Delete(ctx, key, clientv3.WithRange(key+"z")); err == nil {
			return
		}
	}
}

func getWithRetry(client *clientv3.Client, ctx context.Context, key string) *clientv3.GetResponse {
	for {
		if gr, err := client.Get(ctx, key); err == nil {
			return gr
		}
	}
}
