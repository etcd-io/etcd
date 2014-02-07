package raft

import (
	"io"
	"io/ioutil"

	"github.com/coreos/etcd/third_party/code.google.com/p/gogoprotobuf/proto"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft/protobuf"
)

// The request sent to a server to append entries to the log.
type AppendEntriesRequest struct {
	Term		uint64
	PrevLogIndex	uint64
	PrevLogTerm	uint64
	CommitIndex	uint64
	LeaderName	string
	Entries		[]*protobuf.LogEntry
}

// Creates a new AppendEntries request.
func newAppendEntriesRequest(term uint64, prevLogIndex uint64, prevLogTerm uint64,
	commitIndex uint64, leaderName string, entries []*LogEntry) *AppendEntriesRequest {
	pbEntries := make([]*protobuf.LogEntry, len(entries))

	for i := range entries {
		pbEntries[i] = entries[i].pb
	}

	return &AppendEntriesRequest{
		Term:		term,
		PrevLogIndex:	prevLogIndex,
		PrevLogTerm:	prevLogTerm,
		CommitIndex:	commitIndex,
		LeaderName:	leaderName,
		Entries:	pbEntries,
	}
}

// Encodes the AppendEntriesRequest to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (req *AppendEntriesRequest) Encode(w io.Writer) (int, error) {
	pb := &protobuf.AppendEntriesRequest{
		Term:		proto.Uint64(req.Term),
		PrevLogIndex:	proto.Uint64(req.PrevLogIndex),
		PrevLogTerm:	proto.Uint64(req.PrevLogTerm),
		CommitIndex:	proto.Uint64(req.CommitIndex),
		LeaderName:	proto.String(req.LeaderName),
		Entries:	req.Entries,
	}

	p, err := proto.Marshal(pb)
	if err != nil {
		return -1, err
	}

	return w.Write(p)
}

// Decodes the AppendEntriesRequest from a buffer. Returns the number of bytes read and
// any error that occurs.
func (req *AppendEntriesRequest) Decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return -1, err
	}

	pb := new(protobuf.AppendEntriesRequest)
	if err := proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	req.Term = pb.GetTerm()
	req.PrevLogIndex = pb.GetPrevLogIndex()
	req.PrevLogTerm = pb.GetPrevLogTerm()
	req.CommitIndex = pb.GetCommitIndex()
	req.LeaderName = pb.GetLeaderName()
	req.Entries = pb.GetEntries()

	return len(data), nil
}
