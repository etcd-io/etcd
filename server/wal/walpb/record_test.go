package walpb

import (
	"testing"

	"github.com/golang/protobuf/descriptor"
	"go.etcd.io/etcd/raft/v3/raftpb"
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
