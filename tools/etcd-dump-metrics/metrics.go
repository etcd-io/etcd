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
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	"go.etcd.io/etcd/pkg/transport"

	"go.uber.org/zap"
)

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
