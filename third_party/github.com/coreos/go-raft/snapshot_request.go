package raft

import (
	"code.google.com/p/goprotobuf/proto"
	"github.com/coreos/go-raft/protobuf"
	"io"
	"io/ioutil"
)

// The request sent to a server to start from the snapshot.
type SnapshotRequest struct {
	LeaderName string
	LastIndex  uint64
	LastTerm   uint64
}

//------------------------------------------------------------------------------
//
// Constructors
//
//------------------------------------------------------------------------------

// Creates a new Snapshot request.
func newSnapshotRequest(leaderName string, snapshot *Snapshot) *SnapshotRequest {
	return &SnapshotRequest{
		LeaderName: leaderName,
		LastIndex:  snapshot.LastIndex,
		LastTerm:   snapshot.LastTerm,
	}
}

// Encodes the SnapshotRequest to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (req *SnapshotRequest) encode(w io.Writer) (int, error) {
	pb := &protobuf.ProtoSnapshotRequest{
		LeaderName: proto.String(req.LeaderName),
		LastIndex:  proto.Uint64(req.LastIndex),
		LastTerm:   proto.Uint64(req.LastTerm),
	}
	p, err := proto.Marshal(pb)
	if err != nil {
		return -1, err
	}

	return w.Write(p)
}

// Decodes the SnapshotRequest from a buffer. Returns the number of bytes read and
// any error that occurs.
func (req *SnapshotRequest) decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return 0, err
	}

	totalBytes := len(data)

	pb := &protobuf.ProtoSnapshotRequest{}

	if err := proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	req.LeaderName = pb.GetLeaderName()
	req.LastIndex = pb.GetLastIndex()
	req.LastTerm = pb.GetLastTerm()

	return totalBytes, nil
}
