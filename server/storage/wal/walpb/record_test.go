// Copyright 2022 The etcd Authors
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

package walpb

import (
	"testing"

	"github.com/golang/protobuf/descriptor"

	"go.etcd.io/raft/v3/raftpb"
)

func TestSnapshotMetadataCompatibility(t *testing.T) {
	_, snapshotMetadataMd := descriptor.ForMessage(&raftpb.SnapshotMetadata{})
	_, snapshotMd := descriptor.ForMessage(&Snapshot{})
	if len(snapshotMetadataMd.GetField()) != len(snapshotMd.GetField()) {
		t.Errorf("Different number of fields in raftpb.SnapshotMetadata vs. walpb.Snapshot. " +
			"They are supposed to be in sync.")
	}
}

func TestValidateSnapshot(t *testing.T) {
	tests := []struct {
		name    string
		snap    *Snapshot
		wantErr bool
	}{
		{name: "empty", snap: &Snapshot{}, wantErr: false},
		{name: "invalid", snap: &Snapshot{Index: 5, Term: 3}, wantErr: true},
		{name: "valid", snap: &Snapshot{Index: 5, Term: 3, ConfState: &raftpb.ConfState{Voters: []uint64{0x00cad1}}}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateSnapshotForWrite(tt.snap); (err != nil) != tt.wantErr {
				t.Errorf("ValidateSnapshotForWrite() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
