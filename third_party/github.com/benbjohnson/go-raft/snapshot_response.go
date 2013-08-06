package raft

import (
	"code.google.com/p/goprotobuf/proto"
	"github.com/benbjohnson/go-raft/protobuf"
	"io"
	"io/ioutil"
)

// The response returned if the follower entered snapshot state
type SnapshotResponse struct {
	Success bool `json:"success"`
}

//------------------------------------------------------------------------------
//
// Constructors
//
//------------------------------------------------------------------------------

// Creates a new Snapshot response.
func newSnapshotResponse(success bool) *SnapshotResponse {
	return &SnapshotResponse{
		Success: success,
	}
}

// Encodes the SnapshotResponse to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (resp *SnapshotResponse) encode(w io.Writer) (int, error) {
	pb := &protobuf.ProtoSnapshotResponse{
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
func (resp *SnapshotResponse) decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return 0, err
	}

	totalBytes := len(data)

	pb := &protobuf.ProtoSnapshotResponse{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	resp.Success = pb.GetSuccess()

	return totalBytes, nil
}
