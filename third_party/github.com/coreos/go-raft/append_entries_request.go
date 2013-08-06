package raft

import (
	"code.google.com/p/goprotobuf/proto"
	"github.com/coreos/go-raft/protobuf"
	"io"
	"io/ioutil"
)

// The request sent to a server to append entries to the log.
type AppendEntriesRequest struct {
	Term         uint64
	PrevLogIndex uint64
	PrevLogTerm  uint64
	CommitIndex  uint64
	LeaderName   string
	Entries      []*LogEntry
}

// Creates a new AppendEntries request.
func newAppendEntriesRequest(term uint64, prevLogIndex uint64, prevLogTerm uint64, commitIndex uint64, leaderName string, entries []*LogEntry) *AppendEntriesRequest {
	return &AppendEntriesRequest{
		Term:         term,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		CommitIndex:  commitIndex,
		LeaderName:   leaderName,
		Entries:      entries,
	}
}

// Encodes the AppendEntriesRequest to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (req *AppendEntriesRequest) encode(w io.Writer) (int, error) {

	protoEntries := make([]*protobuf.ProtoAppendEntriesRequest_ProtoLogEntry, len(req.Entries))

	for i, entry := range req.Entries {
		protoEntries[i] = &protobuf.ProtoAppendEntriesRequest_ProtoLogEntry{
			Index:       proto.Uint64(entry.Index),
			Term:        proto.Uint64(entry.Term),
			CommandName: proto.String(entry.CommandName),
			Command:     entry.Command,
		}
	}

	pb := &protobuf.ProtoAppendEntriesRequest{
		Term:         proto.Uint64(req.Term),
		PrevLogIndex: proto.Uint64(req.PrevLogIndex),
		PrevLogTerm:  proto.Uint64(req.PrevLogTerm),
		CommitIndex:  proto.Uint64(req.CommitIndex),
		LeaderName:   proto.String(req.LeaderName),
		Entries:      protoEntries,
	}

	p, err := proto.Marshal(pb)
	if err != nil {
		return -1, err
	}

	return w.Write(p)
}

// Decodes the AppendEntriesRequest from a buffer. Returns the number of bytes read and
// any error that occurs.
func (req *AppendEntriesRequest) decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return -1, err
	}

	totalBytes := len(data)

	pb := &protobuf.ProtoAppendEntriesRequest{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	req.Term = pb.GetTerm()
	req.PrevLogIndex = pb.GetPrevLogIndex()
	req.PrevLogTerm = pb.GetPrevLogTerm()
	req.CommitIndex = pb.GetCommitIndex()
	req.LeaderName = pb.GetLeaderName()

	req.Entries = make([]*LogEntry, len(pb.Entries))

	for i, entry := range pb.Entries {
		req.Entries[i] = &LogEntry{
			Index:       entry.GetIndex(),
			Term:        entry.GetTerm(),
			CommandName: entry.GetCommandName(),
			Command:     entry.Command,
		}
	}

	return totalBytes, nil
}
