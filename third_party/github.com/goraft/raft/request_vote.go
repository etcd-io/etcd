package raft

import (
	"io"
	"io/ioutil"

	"github.com/coreos/etcd/third_party/code.google.com/p/gogoprotobuf/proto"
	"github.com/coreos/etcd/third_party/github.com/goraft/raft/protobuf"
)

// The request sent to a server to vote for a candidate to become a leader.
type RequestVoteRequest struct {
	peer          *Peer
	Term          uint64
	LastLogIndex  uint64
	LastLogTerm   uint64
	CandidateName string
}

// The response returned from a server after a vote for a candidate to become a leader.
type RequestVoteResponse struct {
	peer        *Peer
	Term        uint64
	VoteGranted bool
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
func (req *RequestVoteRequest) Encode(w io.Writer) (int, error) {
	pb := &protobuf.RequestVoteRequest{
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
func (req *RequestVoteRequest) Decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return -1, err
	}

	totalBytes := len(data)

	pb := &protobuf.RequestVoteRequest{}
	if err = proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	req.Term = pb.GetTerm()
	req.LastLogIndex = pb.GetLastLogIndex()
	req.LastLogTerm = pb.GetLastLogTerm()
	req.CandidateName = pb.GetCandidateName()

	return totalBytes, nil
}

// Creates a new RequestVote response.
func newRequestVoteResponse(term uint64, voteGranted bool) *RequestVoteResponse {
	return &RequestVoteResponse{
		Term:        term,
		VoteGranted: voteGranted,
	}
}

// Encodes the RequestVoteResponse to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (resp *RequestVoteResponse) Encode(w io.Writer) (int, error) {
	pb := &protobuf.RequestVoteResponse{
		Term:        proto.Uint64(resp.Term),
		VoteGranted: proto.Bool(resp.VoteGranted),
	}

	p, err := proto.Marshal(pb)
	if err != nil {
		return -1, err
	}

	return w.Write(p)
}

// Decodes the RequestVoteResponse from a buffer. Returns the number of bytes read and
// any error that occurs.
func (resp *RequestVoteResponse) Decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return 0, err
	}

	totalBytes := len(data)

	pb := &protobuf.RequestVoteResponse{}
	if err = proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	resp.Term = pb.GetTerm()
	resp.VoteGranted = pb.GetVoteGranted()

	return totalBytes, nil
}
