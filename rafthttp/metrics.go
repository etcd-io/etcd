// Copyright 2015 CoreOS, Inc.
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

package rafthttp

import (
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/prometheus/client_golang/prometheus"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

var (
	encodeDuration = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "rafthttp_stream_encode_microseconds",
			Help: "encode message latency distributions.",
		},
		[]string{"streamType", "to", "msgType"},
	)

	decodeDuration = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "rafthttp_stream_decode_microseconds",
			Help: "decode message latency distributions.",
		},
		[]string{"streamType", "from", "msgType"},
	)
)

func init() {
	prometheus.MustRegister(encodeDuration)
	prometheus.MustRegister(decodeDuration)
}

func reportEncodeDuration(stream streamType, m raftpb.Message, duration time.Duration) {
	typ := m.Type.String()
	// TODO: snapshot?
	if isLinkHeartbeatMessage(m) {
		typ = "MsgLinkHeartbeat"
	}
	encodeDuration.WithLabelValues(string(stream), types.ID(m.To).String(), typ).Observe(float64(duration.Nanoseconds() / int64(time.Microsecond)))
}

func reportDecodeDuration(stream streamType, m raftpb.Message, duration time.Duration) {
	typ := m.Type.String()
	// TODO: snapshot?
	if isLinkHeartbeatMessage(m) {
		typ = "MsgLinkHeartbeat"
	}
	decodeDuration.WithLabelValues(string(stream), types.ID(m.From).String(), typ).Observe(float64(duration.Nanoseconds() / int64(time.Microsecond)))
}
