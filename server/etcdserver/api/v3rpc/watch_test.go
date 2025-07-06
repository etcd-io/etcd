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

package v3rpc

import (
	"bytes"
	"math"
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

func TestSendFragment(t *testing.T) {
	tt := []struct {
		wr              *pb.WatchResponse
		maxRequestBytes int
		fragments       int
		werr            error
	}{
		{ // large limit should not fragment
			wr:              createResponse(100, 1),
			maxRequestBytes: math.MaxInt32,
			fragments:       1,
		},
		{ // large limit for two messages, expect no fragment
			wr:              createResponse(10, 2),
			maxRequestBytes: 50,
			fragments:       1,
		},
		{ // limit is small but only one message, expect no fragment
			wr:              createResponse(1024, 1),
			maxRequestBytes: 1,
			fragments:       1,
		},
		{ // exceed limit only when combined, expect fragments
			wr:              createResponse(11, 5),
			maxRequestBytes: 20,
			fragments:       5,
		},
		{ // 5 events with each event exceeding limits, expect fragments
			wr:              createResponse(15, 5),
			maxRequestBytes: 10,
			fragments:       5,
		},
		{ // 4 events with some combined events exceeding limits
			wr:              createResponse(10, 4),
			maxRequestBytes: 35,
			fragments:       2,
		},
	}

	for i := range tt {
		fragmentedResp := make([]*pb.WatchResponse, 0)
		testSend := func(wr *pb.WatchResponse) error {
			fragmentedResp = append(fragmentedResp, wr)
			return nil
		}
		err := sendFragments(tt[i].wr, tt[i].maxRequestBytes, testSend)
		if err != tt[i].werr {
			t.Errorf("#%d: expected error %v, got %v", i, tt[i].werr, err)
		}
		got := len(fragmentedResp)
		if got != tt[i].fragments {
			t.Errorf("#%d: expected response number %d, got %d", i, tt[i].fragments, got)
		}
		if got > 0 && fragmentedResp[got-1].Fragment {
			t.Errorf("#%d: expected fragment=false in last response, got %+v", i, fragmentedResp[got-1])
		}
	}
}

func createResponse(dataSize, events int) (resp *pb.WatchResponse) {
	resp = &pb.WatchResponse{Events: make([]*mvccpb.Event, events)}
	for i := range resp.Events {
		resp.Events[i] = &mvccpb.Event{
			Kv: &mvccpb.KeyValue{
				Key: bytes.Repeat([]byte("a"), dataSize),
			},
		}
	}
	return resp
}
