// Copyright 2017 The etcd Authors
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

package command

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	v3 "go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/pkg/report"

	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
	"gopkg.in/cheggaaa/pb.v1"
)

var (
	checkPerfLoad        string
	checkPerfPrefix      string
	checkDatascaleLoad   string
	checkDatascalePrefix string
	autoCompact          bool
	autoDefrag           bool
)

type checkPerfCfg struct {
	limit    int
	clients  int
	duration int
}

var checkPerfCfgMap = map[string]checkPerfCfg{
	// TODO: support read limit
	"s": {
		limit:    150,
		clients:  50,
		duration: 60,
	},
	"m": {
		limit:    1000,
		clients:  200,
		duration: 60,
	},
	"l": {
		limit:    8000,
		clients:  500,
		duration: 60,
	},
	"xl": {
		limit:    15000,
		clients:  1000,
		duration: 60,
	},
}

type checkDatascaleCfg struct {
	limit   int
	kvSize  int
	clients int
}

var checkDatascaleCfgMap = map[string]checkDatascaleCfg{
	"s": {
		limit:   10000,
		kvSize:  1024,
		clients: 50,
	},
	"m": {
		limit:   100000,
		kvSize:  1024,
		clients: 200,
	},
	"l": {
		limit:   1000000,
		kvSize:  1024,
		clients: 500,
	},
	"xl": {
		// xl tries to hit the upper bound aggressively which is 3 versions of 1M objects (3M in total)
		limit:   3000000,
		kvSize:  1024,
		clients: 1000,
	},
}

// NewCheckCommand returns the cobra command for "check".
func NewCheckCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "check <subcommand>",
		Short: "commands for checking properties of the etcd cluster",
	}

	cc.AddCommand(NewCheckPerfCommand())
	cc.AddCommand(NewCheckDatascaleCommand())

	return cc
}

// NewCheckPerfCommand returns the cobra command for "check perf".
func NewCheckPerfCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "perf [options]",
		Short: "Check the performance of the etcd cluster",
		Run:   newCheckPerfCommand,
	}

	// TODO: support customized configuration
	cmd.Flags().StringVar(&checkPerfLoad, "load", "s", "The performance check's workload model. Accepted workloads: s(small), m(medium), l(large), xl(xLarge)")
	cmd.Flags().StringVar(&checkPerfPrefix, "prefix", "/etcdctl-check-perf/", "The prefix for writing the performance check's keys.")
	cmd.Flags().BoolVar(&autoCompact, "auto-compact", false, "Compact storage with last revision after test is finished.")
	cmd.Flags().BoolVar(&autoDefrag, "auto-defrag", false, "Defragment storage after test is finished.")

	return cmd
}

// newCheckPerfCommand executes the "check perf" command.
func newCheckPerfCommand(cmd *cobra.Command, args []string) {
	var checkPerfAlias = map[string]string{
		"s": "s", "small": "s",
		"m": "m", "medium": "m",
		"l": "l", "large": "l",
		"xl": "xl", "xLarge": "xl",
	}

	model, ok := checkPerfAlias[checkPerfLoad]
	if !ok {
		ExitWithError(ExitBadFeature, fmt.Errorf("unknown load option %v", checkPerfLoad))
	}
	cfg := checkPerfCfgMap[model]

	requests := make(chan v3.Op, cfg.clients)
	limit := rate.NewLimiter(rate.Limit(cfg.limit), 1)

	cc := clientConfigFromCmd(cmd)
	clients := make([]*v3.Client, cfg.clients)
	for i := 0; i < cfg.clients; i++ {
		clients[i] = cc.mustClient()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.duration)*time.Second)
	resp, err := clients[0].Get(ctx, checkPerfPrefix, v3.WithPrefix(), v3.WithLimit(1))
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	if len(resp.Kvs) > 0 {
		ExitWithError(ExitInvalidInput, fmt.Errorf("prefix %q has keys. Delete with etcdctl del --prefix %s first", checkPerfPrefix, checkPerfPrefix))
	}

	ksize, vsize := 256, 1024
	k, v := make([]byte, ksize), string(make([]byte, vsize))

	bar := pb.New(cfg.duration)
	bar.Format("Bom !")
	bar.Start()

	r := report.NewReport("%4.4f")
	var wg sync.WaitGroup

	wg.Add(len(clients))
	for i := range clients {
		go func(c *v3.Client) {
			defer wg.Done()
			for op := range requests {
				st := time.Now()
				_, derr := c.Do(context.Background(), op)
				r.Results() <- report.Result{Err: derr, Start: st, End: time.Now()}
			}
		}(clients[i])
	}

	go func() {
		cctx, ccancel := context.WithTimeout(context.Background(), time.Duration(cfg.duration)*time.Second)
		defer ccancel()
		for limit.Wait(cctx) == nil {
			binary.PutVarint(k, rand.Int63n(math.MaxInt64))
			requests <- v3.OpPut(checkPerfPrefix+string(k), v)
		}
		close(requests)
	}()

	go func() {
		for i := 0; i < cfg.duration; i++ {
			time.Sleep(time.Second)
			bar.Add(1)
		}
		bar.Finish()
	}()

	sc := r.Stats()
	wg.Wait()
	close(r.Results())

	s := <-sc

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	dresp, err := clients[0].Delete(ctx, checkPerfPrefix, v3.WithPrefix())
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}

	if autoCompact {
		compact(clients[0], dresp.Header.Revision)
	}

	if autoDefrag {
		for _, ep := range clients[0].Endpoints() {
			defrag(clients[0], ep)
		}
	}

	ok = true
	if len(s.ErrorDist) != 0 {
		fmt.Println("FAIL: too many errors")
		for k, v := range s.ErrorDist {
			fmt.Printf("FAIL: ERROR(%v) -> %d\n", k, v)
		}
		ok = false
	}

	if s.RPS/float64(cfg.limit) <= 0.9 {
		fmt.Printf("FAIL: Throughput too low: %d writes/s\n", int(s.RPS)+1)
		ok = false
	} else {
		fmt.Printf("PASS: Throughput is %d writes/s\n", int(s.RPS)+1)
	}
	if s.Slowest > 0.5 { // slowest request > 500ms
		fmt.Printf("Slowest request took too long: %fs\n", s.Slowest)
		ok = false
	} else {
		fmt.Printf("PASS: Slowest request took %fs\n", s.Slowest)
	}
	if s.Stddev > 0.1 { // stddev > 100ms
		fmt.Printf("Stddev too high: %fs\n", s.Stddev)
		ok = false
	} else {
		fmt.Printf("PASS: Stddev is %fs\n", s.Stddev)
	}

	if ok {
		fmt.Println("PASS")
	} else {
		fmt.Println("FAIL")
		os.Exit(ExitError)
	}
}

// NewCheckDatascaleCommand returns the cobra command for "check datascale".
func NewCheckDatascaleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "datascale [options]",
		Short: "Check the memory usage of holding data for different workloads on a given server endpoint.",
		Long:  "If no endpoint is provided, localhost will be used. If multiple endpoints are provided, first endpoint will be used.",
		Run:   newCheckDatascaleCommand,
	}

	cmd.Flags().StringVar(&checkDatascaleLoad, "load", "s", "The datascale check's workload model. Accepted workloads: s(small), m(medium), l(large), xl(xLarge)")
	cmd.Flags().StringVar(&checkDatascalePrefix, "prefix", "/etcdctl-check-datascale/", "The prefix for writing the datascale check's keys.")
	cmd.Flags().BoolVar(&autoCompact, "auto-compact", false, "Compact storage with last revision after test is finished.")
	cmd.Flags().BoolVar(&autoDefrag, "auto-defrag", false, "Defragment storage after test is finished.")

	return cmd
}

// newCheckDatascaleCommand executes the "check datascale" command.
func newCheckDatascaleCommand(cmd *cobra.Command, args []string) {
	var checkDatascaleAlias = map[string]string{
		"s": "s", "small": "s",
		"m": "m", "medium": "m",
		"l": "l", "large": "l",
		"xl": "xl", "xLarge": "xl",
	}

	model, ok := checkDatascaleAlias[checkDatascaleLoad]
	if !ok {
		ExitWithError(ExitBadFeature, fmt.Errorf("unknown load option %v", checkDatascaleLoad))
	}
	cfg := checkDatascaleCfgMap[model]

	requests := make(chan v3.Op, cfg.clients)

	cc := clientConfigFromCmd(cmd)
	clients := make([]*v3.Client, cfg.clients)
	for i := 0; i < cfg.clients; i++ {
		clients[i] = cc.mustClient()
	}

	// get endpoints
	eps, errEndpoints := endpointsFromCmd(cmd)
	if errEndpoints != nil {
		ExitWithError(ExitError, errEndpoints)
	}

	ctx, cancel := context.WithCancel(context.Background())
	resp, err := clients[0].Get(ctx, checkDatascalePrefix, v3.WithPrefix(), v3.WithLimit(1))
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	if len(resp.Kvs) > 0 {
		ExitWithError(ExitInvalidInput, fmt.Errorf("prefix %q has keys. Delete with etcdctl del --prefix %s first", checkDatascalePrefix, checkDatascalePrefix))
	}

	ksize, vsize := 512, 512
	k, v := make([]byte, ksize), string(make([]byte, vsize))

	r := report.NewReport("%4.4f")
	var wg sync.WaitGroup
	wg.Add(len(clients))

	// get the process_resident_memory_bytes and process_virtual_memory_bytes before the put operations
	bytesBefore := endpointMemoryMetrics(eps[0])
	if bytesBefore == 0 {
		fmt.Println("FAIL: Could not read process_resident_memory_bytes before the put operations.")
		os.Exit(ExitError)
	}

	fmt.Println(fmt.Sprintf("Start data scale check for work load [%v key-value pairs, %v bytes per key-value, %v concurrent clients].", cfg.limit, cfg.kvSize, cfg.clients))
	bar := pb.New(cfg.limit)
	bar.Format("Bom !")
	bar.Start()

	for i := range clients {
		go func(c *v3.Client) {
			defer wg.Done()
			for op := range requests {
				st := time.Now()
				_, derr := c.Do(context.Background(), op)
				r.Results() <- report.Result{Err: derr, Start: st, End: time.Now()}
				bar.Increment()
			}
		}(clients[i])
	}

	go func() {
		for i := 0; i < cfg.limit; i++ {
			binary.PutVarint(k, rand.Int63n(math.MaxInt64))
			requests <- v3.OpPut(checkDatascalePrefix+string(k), v)
		}
		close(requests)
	}()

	sc := r.Stats()
	wg.Wait()
	close(r.Results())
	bar.Finish()
	s := <-sc

	// get the process_resident_memory_bytes after the put operations
	bytesAfter := endpointMemoryMetrics(eps[0])
	if bytesAfter == 0 {
		fmt.Println("FAIL: Could not read process_resident_memory_bytes after the put operations.")
		os.Exit(ExitError)
	}

	// delete the created kv pairs
	ctx, cancel = context.WithCancel(context.Background())
	dresp, derr := clients[0].Delete(ctx, checkDatascalePrefix, v3.WithPrefix())
	defer cancel()
	if derr != nil {
		ExitWithError(ExitError, derr)
	}

	if autoCompact {
		compact(clients[0], dresp.Header.Revision)
	}

	if autoDefrag {
		for _, ep := range clients[0].Endpoints() {
			defrag(clients[0], ep)
		}
	}

	if bytesAfter == 0 {
		fmt.Println("FAIL: Could not read process_resident_memory_bytes after the put operations.")
		os.Exit(ExitError)
	}

	bytesUsed := bytesAfter - bytesBefore
	mbUsed := bytesUsed / (1024 * 1024)

	if len(s.ErrorDist) != 0 {
		fmt.Println("FAIL: too many errors")
		for k, v := range s.ErrorDist {
			fmt.Printf("FAIL: ERROR(%v) -> %d\n", k, v)
		}
		os.Exit(ExitError)
	} else {
		fmt.Println(fmt.Sprintf("PASS: Approximate system memory used : %v MB.", strconv.FormatFloat(mbUsed, 'f', 2, 64)))
	}
}
