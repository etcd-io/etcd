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

package runner

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

//RunWatch performs performWatchOnPrefixes for n rounds
func RunWatch(cf *WatchRunnerConfig) {
	ctx := context.Background()
	for round := 0; round < cf.Rounds; round++ {
		log.Printf("round %v", round)
		// generate and put keys in cluster
		limiter := rate.NewLimiter(rate.Limit(cf.ReqRate), cf.ReqRate)
		if err := RunWatchOnce(ctx, cf, limiter, round); err != nil {
			log.Fatalf("RunWatch error (%v)", err)
		}
	}
}

// RunWatchOnce performs performWatchOnPrefixes for a specifc round
func RunWatchOnce(ctx context.Context, cf *WatchRunnerConfig, limiter *rate.Limiter, round int) error {
	return performWatchOnPrefixes(ctx, cf, limiter, round)
}

func performWatchOnPrefixes(ctx context.Context, cf *WatchRunnerConfig, limiter *rate.Limiter, round int) error {
	keysPerPrefix := cf.TotalKeys / cf.NumPrefixes
	prefixes := stringutil.UniqueStrings(5, cf.NumPrefixes)
	keys := stringutil.RandomStrings(10, keysPerPrefix)
	roundPrefix := fmt.Sprintf("%16x", round)

	var (
		revision int64
		wg       sync.WaitGroup
		gr       *clientv3.GetResponse
		err      error
	)

	client := newClient(cf.Eps, cf.DialTimeout)
	defer client.Close()

	gr, err = getKey(ctx, client, "non-existent")
	if err != nil {
		return fmt.Errorf("failed to get the initial revision: %v", err)
	}

	revision = gr.Header.Revision

	ctxt, cancel := context.WithDeadline(ctx, time.Now().Add(cf.RunningTime*time.Second))
	defer cancel()

	perrch := make(chan error, 0)

	go func() {
		for _, key := range keys {
			for _, prefix := range prefixes {
				if err = limiter.Wait(ctxt); err != nil {
					return
				}
				if err = putKeyAtMostOnce(ctxt, client, roundPrefix+"-"+prefix+"-"+key); err != nil {
					perrch <- fmt.Errorf("failed to put key: %v", err)
					return
				}
			}
		}
	}()

	ctxc, cancelc := context.WithCancel(ctx)

	wcs := make([]clientv3.WatchChan, 0)
	rcs := make([]*clientv3.Client, 0)

	// close clients
	defer func() {
		for _, rc := range rcs {
			rc.Close()
		}
	}()

	cerrch := make(chan error, len(prefixes)*cf.WatchesPerPrefix)
	for _, prefix := range prefixes {
		for j := 0; j < cf.WatchesPerPrefix; j++ {
			rc := newClient(cf.Eps, cf.DialTimeout)
			rcs = append(rcs, rc)
			watchPrefix := roundPrefix + "-" + prefix
			wc := rc.Watch(ctxc, watchPrefix, clientv3.WithPrefix(), clientv3.WithRev(revision))
			wcs = append(wcs, wc)
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := checkWatchResponse(wc, watchPrefix, keys); err != nil {
					cerrch <- err
				}
			}()
		}
	}

	wg.Wait()
	cancelc()

	// verify all watch channels are closed
	for e, wc := range wcs {
		if _, ok := <-wc; ok {
			return fmt.Errorf("expected wc to be closed, but received %v", e)
		}
	}

	if err = deletePrefix(ctx, client, roundPrefix); err != nil {
		return fmt.Errorf("failed to clean up keys after test: %v", err)
	}

	select {
	case perr := <-perrch:
		return perr
	case cerr := <-cerrch:
		return cerr
	default:
	}
	return nil
}

func checkWatchResponse(wc clientv3.WatchChan, prefix string, keys []string) error {
	for n := 0; n < len(keys); {
		wr, more := <-wc
		if !more {
			return fmt.Errorf("expect more keys (received %d/%d) for %s", len(keys), n, prefix)
		}
		for _, event := range wr.Events {
			expectedKey := prefix + "-" + keys[n]
			receivedKey := string(event.Kv.Key)
			if expectedKey != receivedKey {
				return fmt.Errorf("expected key %q, got %q for prefix : %q", expectedKey, receivedKey, prefix)
			}
			n++
		}
	}
	return nil
}

func putKeyAtMostOnce(ctx context.Context, client *clientv3.Client, key string) error {
	gr, err := getKey(ctx, client, key)
	if err != nil {
		return err
	}

	var modrev int64
	if len(gr.Kvs) > 0 {
		modrev = gr.Kvs[0].ModRevision
	}

	for ctx.Err() == nil {
		_, err := client.Txn(ctx).If(clientv3.Compare(clientv3.ModRevision(key), "=", modrev)).Then(clientv3.OpPut(key, key)).Commit()

		if err == nil {
			return nil
		}
	}

	return ctx.Err()
}

func deletePrefix(ctx context.Context, client *clientv3.Client, key string) error {
	for ctx.Err() == nil {
		if _, err := client.Delete(ctx, key, clientv3.WithPrefix()); err == nil {
			return nil
		}
	}
	return ctx.Err()
}

func getKey(ctx context.Context, client *clientv3.Client, key string) (*clientv3.GetResponse, error) {
	for ctx.Err() == nil {
		if gr, err := client.Get(ctx, key); err == nil {
			return gr, nil
		}
	}
	return nil, ctx.Err()
}
