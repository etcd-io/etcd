package raft

import (
	"io"
	"io/ioutil"

	"github.com/coreos/etcd/third_party/code.google.com/p/gogoprotobuf/proto"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft/protobuf"
)

// The response returned from a server appending entries to the log.
type AppendEntriesResponse struct {
	pb	*protobuf.AppendEntriesResponse
	peer	string
	append	bool
}

// Creates a new AppendEntries response.
func newAppendEntriesResponse(term uint64, success bool, index uint64, commitIndex uint64) *AppendEntriesResponse {
	pb := &protobuf.AppendEntriesResponse{
		Term:		proto.Uint64(term),
		Index:		proto.Uint64(index),
		Success:	proto.Bool(success),
		CommitIndex:	proto.Uint64(commitIndex),
	}

	return &AppendEntriesResponse{
		pb: pb,
	}
}

func (aer *AppendEntriesResponse) Index() uint64 {
	return aer.pb.GetIndex()
}

func (aer *AppendEntriesResponse) CommitIndex() uint64 {
	return aer.pb.GetCommitIndex()
}

func (aer *AppendEntriesResponse) Term() uint64 {
	return aer.pb.GetTerm()
}

func (aer *AppendEntriesResponse) Success() bool {
	return aer.pb.GetSuccess()
}

// Encodes the AppendEntriesResponse to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (resp *AppendEntriesResponse) Encode(w io.Writer) (int, error) {
	b, err := proto.Marshal(resp.pb)
	if err != nil {
		return -1, err
	}

	return w.Write(b)
}

// Decodes the AppendEntriesResponse from a buffer. Returns the number of bytes read and
// any error that occurs.
func (resp *AppendEntriesResponse) Decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return -1, err
	}

	resp.pb = new(protobuf.AppendEntriesResponse)
	if err := proto.Unmarshal(data, resp.pb); err != nil {
		return -1, err
	}

	return len(data), nil
}
