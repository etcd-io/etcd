package raft

import (
	"code.google.com/p/goprotobuf/proto"
	"github.com/benbjohnson/go-raft/protobuf"
	"io"
	"io/ioutil"
)

// The response returned from a server appending entries to the log.
type SnapshotRecoveryResponse struct {
	Term        uint64
	Success     bool
	CommitIndex uint64
}

//------------------------------------------------------------------------------
//
// Constructors
//
//------------------------------------------------------------------------------

// Creates a new Snapshot response.
func newSnapshotRecoveryResponse(term uint64, success bool, commitIndex uint64) *SnapshotRecoveryResponse {
	return &SnapshotRecoveryResponse{
		Term:        term,
		Success:     success,
		CommitIndex: commitIndex,
	}
}

// Encodes the SnapshotRecoveryResponse to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (req *SnapshotRecoveryResponse) encode(w io.Writer) (int, error) {
	pb := &protobuf.ProtoSnapshotRecoveryResponse{
		Term:        proto.Uint64(req.Term),
		Success:     proto.Bool(req.Success),
		CommitIndex: proto.Uint64(req.CommitIndex),
	}
	p, err := proto.Marshal(pb)
	if err != nil {
		return -1, err
	}

	return w.Write(p)
}

// Decodes the SnapshotRecoveryResponse from a buffer. Returns the number of bytes read and
// any error that occurs.
func (req *SnapshotRecoveryResponse) decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return 0, err
	}

	totalBytes := len(data)

	pb := &protobuf.ProtoSnapshotRecoveryResponse{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	req.Term = pb.GetTerm()
	req.Success = pb.GetSuccess()
	req.CommitIndex = pb.GetCommitIndex()

	return totalBytes, nil
}
