package raft

import (
	"code.google.com/p/goprotobuf/proto"
	"github.com/benbjohnson/go-raft/protobuf"
	"io"
	"io/ioutil"
)

// The request sent to a server to vote for a candidate to become a leader.
type RequestVoteRequest struct {
	peer          *Peer
	Term          uint64
	LastLogIndex  uint64
	LastLogTerm   uint64
	CandidateName string
}

// Creates a new RequestVote request.
func newRequestVoteRequest(term uint64, candidateName string, lastLogIndex uint64, lastLogTerm uint64) *RequestVoteRequest {
	return &RequestVoteRequest{
		Term:          term,
		LastLogIndex:  lastLogIndex,
		LastLogTerm:   lastLogTerm,
		CandidateName: candidateName,
	}
}

// Encodes the RequestVoteRequest to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (req *RequestVoteRequest) encode(w io.Writer) (int, error) {
	pb := &protobuf.ProtoRequestVoteRequest{
		Term:          proto.Uint64(req.Term),
		LastLogIndex:  proto.Uint64(req.LastLogIndex),
		LastLogTerm:   proto.Uint64(req.LastLogTerm),
		CandidateName: proto.String(req.CandidateName),
	}
	p, err := proto.Marshal(pb)
	if err != nil {
		return -1, err
	}

	return w.Write(p)
}

// Decodes the RequestVoteRequest from a buffer. Returns the number of bytes read and
// any error that occurs.
func (req *RequestVoteRequest) decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return -1, err
	}

	totalBytes := len(data)

	pb := &protobuf.ProtoRequestVoteRequest{}
	if err = proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	req.Term = pb.GetTerm()
	req.LastLogIndex = pb.GetLastLogIndex()
	req.LastLogTerm = pb.GetLastLogTerm()
	req.CandidateName = pb.GetCandidateName()

	return totalBytes, nil
}
