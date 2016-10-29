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
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
)

var (
	endpointStr   string
	mode          string
	rounds        int
	clientTimeout int64
	eps           []string
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func main() {
	log.SetFlags(log.Lmicroseconds)

	flag.StringVar(&endpointStr, "endpoints", "localhost:2379", "endpoints of etcd cluster")
	flag.StringVar(&mode, "mode", "watcher", "test mode (election, lock-racer, lease-renewer, watcher)")
	flag.IntVar(&rounds, "rounds", 100, "number of rounds to run")
	flag.Int64Var(&clientTimeout, "client-timeout", 60, "max timeout seconds for a client to get connection")
	flag.Parse()
	eps = strings.Split(endpointStr, ",")

	switch mode {
	case "election":
		runElection()
	case "lock-racer":
		runRacer()
	case "lease-renewer":
		runLeaseRenewer()
	case "watcher":
		runWatcher()
	default:
		fmt.Fprintf(os.Stderr, "unsupported mode %v\n", mode)
	}
}

func runElection() {
	rcs := make([]roundClient, 15)
	validatec, releasec := make(chan struct{}, len(rcs)), make(chan struct{}, len(rcs))
	for range rcs {
		releasec <- struct{}{}
	}

	for i := range rcs {
		v := fmt.Sprintf("%d", i)
		observedLeader := ""
		validateWaiters := 0

		rcs[i].c = newClient()
		var (
			s   *concurrency.Session
			err error
		)
		for {
			s, err = concurrency.NewSession(rcs[i].c)
			if err == nil {
				break
			}
		}
		e := concurrency.NewElection(s, "electors")

		rcs[i].acquire = func() error {
			<-releasec
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				if ol, ok := <-e.Observe(ctx); ok {
					observedLeader = string(ol.Kvs[0].Value)
					if observedLeader != v {
						cancel()
					}
				}
			}()
			err = e.Campaign(ctx, v)
			if err == nil {
				observedLeader = v
			}
			if observedLeader == v {
				validateWaiters = len(rcs)
			}
			select {
			case <-ctx.Done():
				return nil
			default:
				cancel()
				return err
			}
		}
		rcs[i].validate = func() error {
			if l, err := e.Leader(context.TODO()); err == nil && l != observedLeader {
				return fmt.Errorf("expected leader %q, got %q", observedLeader, l)
			}
			validatec <- struct{}{}
			return nil
		}
		rcs[i].release = func() error {
			for validateWaiters > 0 {
				select {
				case <-validatec:
					validateWaiters--
				default:
					return fmt.Errorf("waiting on followers")
				}
			}
			if err := e.Resign(context.TODO()); err != nil {
				return err
			}
			if observedLeader == v {
				for range rcs {
					releasec <- struct{}{}
				}
			}
			observedLeader = ""
			return nil
		}
	}
	doRounds(rcs)
}

func runLeaseRenewer() {
	c := newClient()
	ctx := context.Background()

	for {
		var (
			l   *clientv3.LeaseGrantResponse
			lk  *clientv3.LeaseKeepAliveResponse
			err error
		)
		for {
			l, err = c.Lease.Grant(ctx, 5)
			if err == nil {
				break
			}
		}
		expire := time.Now().Add(time.Duration(l.TTL-1) * time.Second)

		for {
			lk, err = c.Lease.KeepAliveOnce(ctx, l.ID)
			if grpc.Code(err) == codes.NotFound {
				if time.Since(expire) < 0 {
					log.Printf("bad renew! exceeded: %v", time.Since(expire))
					for {
						lk, err = c.Lease.KeepAliveOnce(ctx, l.ID)
						fmt.Println(lk, err)
						time.Sleep(time.Second)
					}
				}
				log.Printf("lost lease %d, expire: %v\n", l.ID, expire)
				break
			}
			if err != nil {
				continue
			}
			expire = time.Now().Add(time.Duration(lk.TTL-1) * time.Second)
			log.Printf("renewed lease %d, expire: %v\n", lk.ID, expire)
			time.Sleep(time.Duration(lk.TTL-2) * time.Second)
		}
	}
}

func runRacer() {
	rcs := make([]roundClient, 15)
	ctx := context.Background()
	cnt := 0
	for i := range rcs {
		rcs[i].c = newClient()
		var (
			s   *concurrency.Session
			err error
		)
		for {
			s, err = concurrency.NewSession(rcs[i].c)
			if err == nil {
				break
			}
		}
		m := concurrency.NewMutex(s, "racers")
		rcs[i].acquire = func() error { return m.Lock(ctx) }
		rcs[i].validate = func() error {
			if cnt++; cnt != 1 {
				return fmt.Errorf("bad lock; count: %d", cnt)
			}
			return nil
		}
		rcs[i].release = func() error {
			if err := m.Unlock(ctx); err != nil {
				return err
			}
			cnt = 0
			return nil
		}
	}
	doRounds(rcs)
}

func runWatcher() {
	ctx := context.Background()
	for round := 0; round < rounds; round++ {
		performWatchOnPrefixes(ctx, round)
	}
}

func performWatchOnPrefixes(ctx context.Context, round int) {
	runningTime := 60 * time.Second // time for which operation should be performed
	noOfPrefixes := 36              // total number of prefixes which will be watched upon
	watchPerPrefix := 10            // number of watchers per prefix
	reqRate := 30                   // put request per second
	keyPrePrefix := 30              // max number of keyPrePrefixs for put operation

	prefixes := generateUniqueKeys(5, noOfPrefixes)
	keys := generateRandomKeys(10, keyPrePrefix)

	roundPrefix := fmt.Sprintf("%16x", round)

	var (
		revision int64
		wg       sync.WaitGroup
		gr       *clientv3.GetResponse
		err      error
	)

	// create client for performing get and put operations
	client := newClient()
	defer client.Close()

	// get revision using get request
	gr = getWithRetry(client, ctx, "non-existant")
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

				rc := newClient()
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

func generateUniqueKeys(maxstrlen uint, keynos int) []string {
	keyMap := make(map[string]bool)
	keys := make([]string, 0)
	count := 0
	key := ""
	for {
		key = generateRandomKey(maxstrlen)
		_, ok := keyMap[key]
		if !ok {
			keyMap[key] = true
			keys = append(keys, key)
			count++
			if len(keys) == keynos {
				break
			}
		}
	}
	return keys
}

func generateRandomKeys(maxstrlen uint, keynos int) []string {
	keys := make([]string, 0)
	key := ""
	for i := 0; i < keynos; i++ {
		key = generateRandomKey(maxstrlen)
		keys = append(keys, key)
	}
	return keys
}

func generateRandomKey(strlen uint) string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, strlen)
	for i := 0; i < int(strlen); i++ {
		result[i] = chars[rand.Intn(len(chars))]
	}
	key := string(result)
	return key
}

func newClient() *clientv3.Client {
	c, err := clientv3.New(clientv3.Config{
		Endpoints:   eps,
		DialTimeout: time.Duration(clientTimeout) * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	return c
}

type roundClient struct {
	c        *clientv3.Client
	progress int
	acquire  func() error
	validate func() error
	release  func() error
}

func doRounds(rcs []roundClient) {
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(len(rcs))
	finished := make(chan struct{}, 0)
	for i := range rcs {
		go func(rc *roundClient) {
			defer wg.Done()
			for rc.progress < rounds {
				for rc.acquire() != nil { /* spin */
				}

				mu.Lock()
				if err := rc.validate(); err != nil {
					log.Fatal(err)
				}
				mu.Unlock()

				time.Sleep(10 * time.Millisecond)
				rc.progress++
				finished <- struct{}{}

				mu.Lock()
				for rc.release() != nil {
					mu.Unlock()
					mu.Lock()
				}
				mu.Unlock()
			}
		}(&rcs[i])
	}

	start := time.Now()
	for i := 1; i < len(rcs)*rounds+1; i++ {
		select {
		case <-finished:
			if i%100 == 0 {
				fmt.Printf("finished %d, took %v\n", i, time.Since(start))
				start = time.Now()
			}
		case <-time.After(time.Minute):
			log.Panic("no progress after 1 minute!")
		}
	}
	wg.Wait()

	for _, rc := range rcs {
		rc.c.Close()
	}
}
