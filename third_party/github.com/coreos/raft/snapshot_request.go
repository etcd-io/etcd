package raft

import (
	"io"
	"io/ioutil"

	"github.com/coreos/etcd/third_party/code.google.com/p/gogoprotobuf/proto"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft/protobuf"
)

// The request sent to a server to start from the snapshot.
type SnapshotRequest struct {
	LeaderName	string
	LastIndex	uint64
	LastTerm	uint64
}

// Creates a new Snapshot request.
func newSnapshotRequest(leaderName string, snapshot *Snapshot) *SnapshotRequest {
	return &SnapshotRequest{
		LeaderName:	leaderName,
		LastIndex:	snapshot.LastIndex,
		LastTerm:	snapshot.LastTerm,
	}
}

// Encodes the SnapshotRequest to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (req *SnapshotRequest) Encode(w io.Writer) (int, error) {
	pb := &protobuf.SnapshotRequest{
		LeaderName:	proto.String(req.LeaderName),
		LastIndex:	proto.Uint64(req.LastIndex),
		LastTerm:	proto.Uint64(req.LastTerm),
	}
	p, err := proto.Marshal(pb)
	if err != nil {
		return -1, err
	}

	return w.Write(p)
}

// Decodes the SnapshotRequest from a buffer. Returns the number of bytes read and
// any error that occurs.
func (req *SnapshotRequest) Decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return 0, err
	}

	totalBytes := len(data)

	pb := &protobuf.SnapshotRequest{}

	if err := proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	req.LeaderName = pb.GetLeaderName()
	req.LastIndex = pb.GetLastIndex()
	req.LastTerm = pb.GetLastTerm()

	return totalBytes, nil
}
