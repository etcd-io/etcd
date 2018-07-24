// Copyright 2018 The etcd Authors
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
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/embed"
	"github.com/coreos/etcd/pkg/transport"

	"go.uber.org/zap"
)

var lg *zap.Logger

func init() {
	var err error
	lg, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}

func main() {
	addr := flag.String("addr", "", "etcd metrics URL to fetch from (empty to use current git branch)")
	enableLog := flag.Bool("server-log", false, "true to enable embedded etcd server logs")
	debug := flag.Bool("debug", false, "true to enable debug logging")
	flag.Parse()

	if *debug {
		lg = zap.NewExample()
	}

	ep := *addr
	if ep == "" {
		uss := newEmbedURLs(4)
		ep = uss[0].String() + "/metrics"

		cfgs := []*embed.Config{embed.NewConfig(), embed.NewConfig()}
		cfgs[0].Name, cfgs[1].Name = "0", "1"
		setupEmbedCfg(cfgs[0], *enableLog, []url.URL{uss[0]}, []url.URL{uss[1]}, []url.URL{uss[1], uss[3]})
		setupEmbedCfg(cfgs[1], *enableLog, []url.URL{uss[2]}, []url.URL{uss[3]}, []url.URL{uss[1], uss[3]})
		type embedAndError struct {
			ec  *embed.Etcd
			err error
		}
		ech := make(chan embedAndError)
		for _, cfg := range cfgs {
			go func(c *embed.Config) {
				e, err := embed.StartEtcd(c)
				if err != nil {
					ech <- embedAndError{err: err}
					return
				}
				<-e.Server.ReadyNotify()
				ech <- embedAndError{ec: e}
			}(cfg)
		}
		for range cfgs {
			ev := <-ech
			if ev.err != nil {
				lg.Panic("failed to start embedded etcd", zap.Error(ev.err))
			}
			defer ev.ec.Close()
		}

		// give enough time for peer-to-peer metrics
		time.Sleep(7 * time.Second)

		lg.Debug("started 2-node embedded etcd cluster")
	}

	lg.Debug("starting etcd-dump-metrics", zap.String("endpoint", ep))

	// send client requests to populate gRPC client-side metrics
	// TODO: enable default metrics initialization in v3.1 and v3.2
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{strings.Replace(ep, "/metrics", "", 1)}})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}
	defer cli.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Put(ctx, "____test", "")
	if err != nil {
		lg.Panic("failed to write test key", zap.Error(err))
	}
	_, err = cli.Get(ctx, "____test")
	if err != nil {
		lg.Panic("failed to read test key", zap.Error(err))
	}
	_, err = cli.Delete(ctx, "____test")
	if err != nil {
		lg.Panic("failed to delete test key", zap.Error(err))
	}
	cli.Watch(ctx, "____test", clientv3.WithCreatedNotify())

	fmt.Println(getMetrics(ep))
}

func getMetrics(ep string) (m metricSlice) {
	lines, err := fetchMetrics(ep)
	if err != nil {
		lg.Panic("failed to fetch metrics", zap.Error(err))
	}
	mss := parse(lines)
	sort.Sort(metricSlice(mss))
	return mss
}

func (mss metricSlice) String() (s string) {
	ver := "unknown"
	for i, v := range mss {
		if strings.HasPrefix(v.name, "etcd_server_version") {
			ver = v.metrics[0]
		}
		s += v.String()
		if i != len(mss)-1 {
			s += "\n\n"
		}
	}
	return "# server version: " + ver + "\n\n" + s
}

type metricSlice []metric

func (mss metricSlice) Len() int {
	return len(mss)
}

func (mss metricSlice) Less(i, j int) bool {
	return mss[i].name < mss[j].name
}

func (mss metricSlice) Swap(i, j int) {
	mss[i], mss[j] = mss[j], mss[i]
}

type metric struct {
	// raw data for debugging purposes
	raw []string

	// metrics name
	name string

	// metrics description
	desc string

	// metrics type
	tp string

	// aggregates of "grpc_server_handled_total"
	grpcCodes []string

	// keep fist 1 and last 4 if histogram or summary
	// otherwise, keep only 1
	metrics []string
}

func (m metric) String() (s string) {
	s += fmt.Sprintf("# name: %q\n", m.name)
	s += fmt.Sprintf("# description: %q\n", m.desc)
	s += fmt.Sprintf("# type: %q\n", m.tp)
	if len(m.grpcCodes) > 0 {
		s += "# gRPC codes: \n"
		for _, c := range m.grpcCodes {
			s += fmt.Sprintf("#  - %q\n", c)
		}
	}
	s += strings.Join(m.metrics, "\n")
	return s
}

func parse(lines []string) (mss []metric) {
	m := metric{raw: make([]string, 0), metrics: make([]string, 0)}
	for _, line := range lines {
		if strings.HasPrefix(line, "# HELP ") {
			// add previous metric and initialize
			if m.name != "" {
				mss = append(mss, m)
			}
			m = metric{raw: make([]string, 0), metrics: make([]string, 0)}

			m.raw = append(m.raw, line)
			ss := strings.Split(strings.Replace(line, "# HELP ", "", 1), " ")
			m.name, m.desc = ss[0], strings.Join(ss[1:], " ")
			continue
		}

		if strings.HasPrefix(line, "# TYPE ") {
			m.raw = append(m.raw, line)
			m.tp = strings.Split(strings.Replace(line, "# TYPE "+m.tp, "", 1), " ")[1]
			continue
		}

		m.raw = append(m.raw, line)
		m.metrics = append(m.metrics, strings.Split(line, " ")[0])
	}
	if m.name != "" {
		mss = append(mss, m)
	}

	// aggregate
	for i := range mss {
		/*
			munge data for:
				etcd_network_active_peers{Local="c6c9b5143b47d146",Remote="fbdddd08d7e1608b"}
				etcd_network_peer_sent_bytes_total{To="c6c9b5143b47d146"}
				etcd_network_peer_received_bytes_total{From="0"}
				etcd_network_peer_received_bytes_total{From="fd422379fda50e48"}
				etcd_network_peer_round_trip_time_seconds_bucket{To="91bc3c398fb3c146",le="0.0001"}
				etcd_network_peer_round_trip_time_seconds_bucket{To="fd422379fda50e48",le="0.8192"}
				etcd_network_peer_round_trip_time_seconds_bucket{To="fd422379fda50e48",le="+Inf"}
				etcd_network_peer_round_trip_time_seconds_sum{To="fd422379fda50e48"}
				etcd_network_peer_round_trip_time_seconds_count{To="fd422379fda50e48"}
		*/
		if mss[i].name == "etcd_network_active_peers" {
			mss[i].metrics = []string{`etcd_network_active_peers{Local="LOCAL_NODE_ID",Remote="REMOTE_PEER_NODE_ID"}`}
		}
		if mss[i].name == "etcd_network_peer_sent_bytes_total" {
			mss[i].metrics = []string{`etcd_network_peer_sent_bytes_total{To="REMOTE_PEER_NODE_ID"}`}
		}
		if mss[i].name == "etcd_network_peer_received_bytes_total" {
			mss[i].metrics = []string{`etcd_network_peer_received_bytes_total{From="REMOTE_PEER_NODE_ID"}`}
		}
		if mss[i].tp == "histogram" || mss[i].tp == "summary" {
			if mss[i].name == "etcd_network_peer_round_trip_time_seconds" {
				for j := range mss[i].metrics {
					l := mss[i].metrics[j]
					if strings.Contains(l, `To="`) && strings.Contains(l, `le="`) {
						k1 := strings.Index(l, `To="`)
						k2 := strings.Index(l, `",le="`)
						mss[i].metrics[j] = l[:k1+4] + "REMOTE_PEER_NODE_ID" + l[k2:]
					}
					if strings.HasPrefix(l, "etcd_network_peer_round_trip_time_seconds_sum") {
						mss[i].metrics[j] = `etcd_network_peer_round_trip_time_seconds_sum{To="REMOTE_PEER_NODE_ID"}`
					}
					if strings.HasPrefix(l, "etcd_network_peer_round_trip_time_seconds_count") {
						mss[i].metrics[j] = `etcd_network_peer_round_trip_time_seconds_count{To="REMOTE_PEER_NODE_ID"}`
					}
				}
				mss[i].metrics = aggSort(mss[i].metrics)
			}
		}

		// aggregate gRPC RPC metrics
		if mss[i].name == "grpc_server_handled_total" {
			pfx := `grpc_server_handled_total{grpc_code="`
			codes, metrics := make(map[string]struct{}), make(map[string]struct{})
			for _, v := range mss[i].metrics {
				v2 := strings.Replace(v, pfx, "", 1)
				idx := strings.Index(v2, `",grpc_method="`)
				code := v2[:idx]
				v2 = v2[idx:]
				codes[code] = struct{}{}
				v2 = pfx + "CODE" + v2
				metrics[v2] = struct{}{}
			}
			mss[i].grpcCodes = sortMap(codes)
			mss[i].metrics = sortMap(metrics)
		}

	}
	return mss
}

func fetchMetrics(ep string) (lines []string, err error) {
	tr, err := transport.NewTimeoutTransport(transport.TLSInfo{}, time.Second, time.Second, time.Second)
	if err != nil {
		return nil, err
	}
	cli := &http.Client{Transport: tr}
	resp, err := cli.Get(ep)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, rerr := ioutil.ReadAll(resp.Body)
	if rerr != nil {
		return nil, rerr
	}
	lines = strings.Split(string(b), "\n")
	return lines, nil
}

func newEmbedURLs(n int) (urls []url.URL) {
	urls = make([]url.URL, n)
	for i := 0; i < n; i++ {
		u, _ := url.Parse(fmt.Sprintf("unix://localhost:%d%06d", os.Getpid(), i))
		urls[i] = *u
	}
	return urls
}

func setupEmbedCfg(cfg *embed.Config, enableLog bool, curls, purls, ics []url.URL) {
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}
	if enableLog {
		cfg.LogOutputs = []string{"stderr"}
	}
	cfg.Debug = false

	var err error
	cfg.Dir, err = ioutil.TempDir(os.TempDir(), fmt.Sprintf("%016X", time.Now().UnixNano()))
	if err != nil {
		panic(err)
	}
	os.RemoveAll(cfg.Dir)

	cfg.ClusterState = "new"
	cfg.LCUrls, cfg.ACUrls = curls, curls
	cfg.LPUrls, cfg.APUrls = purls, purls

	cfg.InitialCluster = ""
	for i := range ics {
		cfg.InitialCluster += fmt.Sprintf(",%d=%s", i, ics[i].String())
	}
	cfg.InitialCluster = cfg.InitialCluster[1:]
}

func aggSort(ss []string) (sorted []string) {
	set := make(map[string]struct{})
	for _, s := range ss {
		set[s] = struct{}{}
	}
	sorted = make([]string, 0, len(set))
	for k := range set {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	return sorted
}

func sortMap(set map[string]struct{}) (sorted []string) {
	sorted = make([]string, 0, len(set))
	for k := range set {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	return sorted
}
