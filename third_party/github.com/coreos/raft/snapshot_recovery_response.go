package raft

import (
	"io"
	"io/ioutil"

	"github.com/coreos/etcd/third_party/code.google.com/p/gogoprotobuf/proto"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft/protobuf"
)

// The response returned from a server appending entries to the log.
type SnapshotRecoveryResponse struct {
	Term		uint64
	Success		bool
	CommitIndex	uint64
}

// Creates a new Snapshot response.
func newSnapshotRecoveryResponse(term uint64, success bool, commitIndex uint64) *SnapshotRecoveryResponse {
	return &SnapshotRecoveryResponse{
		Term:		term,
		Success:	success,
		CommitIndex:	commitIndex,
	}
}

// Encode writes the response to a writer.
// Returns the number of bytes written and any error that occurs.
func (req *SnapshotRecoveryResponse) Encode(w io.Writer) (int, error) {
	pb := &protobuf.SnapshotRecoveryResponse{
		Term:		proto.Uint64(req.Term),
		Success:	proto.Bool(req.Success),
		CommitIndex:	proto.Uint64(req.CommitIndex),
	}
	p, err := proto.Marshal(pb)
	if err != nil {
		return -1, err
	}

	return w.Write(p)
}

// Decodes the SnapshotRecoveryResponse from a buffer.
func (req *SnapshotRecoveryResponse) Decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return 0, err
	}

	totalBytes := len(data)

	pb := &protobuf.SnapshotRecoveryResponse{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	req.Term = pb.GetTerm()
	req.Success = pb.GetSuccess()
	req.CommitIndex = pb.GetCommitIndex()

	return totalBytes, nil
}
