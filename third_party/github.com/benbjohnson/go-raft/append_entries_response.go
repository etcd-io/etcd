package raft

import (
	"code.google.com/p/goprotobuf/proto"
	"github.com/benbjohnson/go-raft/protobuf"
	"io"
	"io/ioutil"
)

// The response returned from a server appending entries to the log.
type AppendEntriesResponse struct {
	Term uint64
	// the current index of the server
	Index       uint64
	Success     bool
	CommitIndex uint64
	peer        string
	append      bool
}

// Creates a new AppendEntries response.
func newAppendEntriesResponse(term uint64, success bool, index uint64, commitIndex uint64) *AppendEntriesResponse {
	return &AppendEntriesResponse{
		Term:        term,
		Success:     success,
		Index:       index,
		CommitIndex: commitIndex,
	}
}

// Encodes the AppendEntriesResponse to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (resp *AppendEntriesResponse) encode(w io.Writer) (int, error) {
	pb := &protobuf.ProtoAppendEntriesResponse{
		Term:        proto.Uint64(resp.Term),
		Index:       proto.Uint64(resp.Index),
		CommitIndex: proto.Uint64(resp.CommitIndex),
		Success:     proto.Bool(resp.Success),
	}
	p, err := proto.Marshal(pb)
	if err != nil {
		return -1, err
	}

	return w.Write(p)
}

// Decodes the AppendEntriesResponse from a buffer. Returns the number of bytes read and
// any error that occurs.
func (resp *AppendEntriesResponse) decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return -1, err
	}

	totalBytes := len(data)

	pb := &protobuf.ProtoAppendEntriesResponse{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	resp.Term = pb.GetTerm()
	resp.Index = pb.GetIndex()
	resp.CommitIndex = pb.GetCommitIndex()
	resp.Success = pb.GetSuccess()

	return totalBytes, nil
}
