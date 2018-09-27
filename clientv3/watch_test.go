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

package clientv3

import (
	"testing"

	"go.etcd.io/etcd/mvcc/mvccpb"
	"go.etcd.io/etcd/pkg/testutil"
)

func TestEvent(t *testing.T) {
	tests := []struct {
		ev       *Event
		isCreate bool
		isModify bool
	}{{
		ev: &Event{
			Type: EventTypePut,
			Kv: &mvccpb.KeyValue{
				CreateRevision: 3,
				ModRevision:    3,
			},
		},
		isCreate: true,
	}, {
		ev: &Event{
			Type: EventTypePut,
			Kv: &mvccpb.KeyValue{
				CreateRevision: 3,
				ModRevision:    4,
			},
		},
		isModify: true,
	}}
	for i, tt := range tests {
		if tt.isCreate && !tt.ev.IsCreate() {
			t.Errorf("#%d: event should be Create event", i)
		}
		if tt.isModify && !tt.ev.IsModify() {
			t.Errorf("#%d: event should be Modify event", i)
		}
	}
}

func TestHasResumingWatchers(t *testing.T) {
	tests := []struct {
		name                string
		wgs                 *watchGrpcStream
		hasResumingWatchers bool
	}{{
		name: "nil stream",
		wgs: &watchGrpcStream{
			resuming: []*watcherStream{
				nil,
			},
		},
		hasResumingWatchers: false,
	}, {
		name: "non-nil stream",
		wgs: &watchGrpcStream{
			resuming: []*watcherStream{
				{},
			},
		},
		hasResumingWatchers: true,
	}, {
		name: "nil stream in the middle",
		wgs: &watchGrpcStream{
			resuming: []*watcherStream{
				{},
				nil,
				{},
			},
		},
		hasResumingWatchers: true,
	}, {
		name: "nil stream at the head",
		wgs: &watchGrpcStream{
			resuming: []*watcherStream{
				nil,
				{},
			},
		},
		hasResumingWatchers: true,
	}, {
		name: "nil stream at the tail",
		wgs: &watchGrpcStream{
			resuming: []*watcherStream{
				{},
				nil,
			},
		},
		hasResumingWatchers: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.AssertEqual(t, tt.hasResumingWatchers, tt.wgs.hasResumingWatchers())
		})
	}
}
