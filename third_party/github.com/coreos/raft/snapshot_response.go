package raft

import (
	"io"
	"io/ioutil"

	"github.com/coreos/etcd/third_party/code.google.com/p/gogoprotobuf/proto"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft/protobuf"
)

// The response returned if the follower entered snapshot state
type SnapshotResponse struct {
	Success bool `json:"success"`
}

// Creates a new Snapshot response.
func newSnapshotResponse(success bool) *SnapshotResponse {
	return &SnapshotResponse{
		Success: success,
	}
}

// Encodes the SnapshotResponse to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (resp *SnapshotResponse) Encode(w io.Writer) (int, error) {
	pb := &protobuf.SnapshotResponse{
		Success: proto.Bool(resp.Success),
	}
	p, err := proto.Marshal(pb)
	if err != nil {
		return -1, err
	}

	return w.Write(p)
}

// Decodes the SnapshotResponse from a buffer. Returns the number of bytes read and
// any error that occurs.
func (resp *SnapshotResponse) Decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return 0, err
	}

	totalBytes := len(data)

	pb := &protobuf.SnapshotResponse{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	resp.Success = pb.GetSuccess()

	return totalBytes, nil
}
