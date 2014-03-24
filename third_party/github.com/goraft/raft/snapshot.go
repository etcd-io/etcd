package raft

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"

	"github.com/coreos/etcd/third_party/code.google.com/p/gogoprotobuf/proto"
	"github.com/coreos/etcd/third_party/github.com/goraft/raft/protobuf"
)

// Snapshot represents an in-memory representation of the current state of the system.
type Snapshot struct {
	LastIndex uint64 `json:"lastIndex"`
	LastTerm  uint64 `json:"lastTerm"`

	// Cluster configuration.
	Peers []*Peer `json:"peers"`
	State []byte  `json:"state"`
	Path  string  `json:"path"`
}

// The request sent to a server to start from the snapshot.
type SnapshotRecoveryRequest struct {
	LeaderName string
	LastIndex  uint64
	LastTerm   uint64
	Peers      []*Peer
	State      []byte
}

// The response returned from a server appending entries to the log.
type SnapshotRecoveryResponse struct {
	Term        uint64
	Success     bool
	CommitIndex uint64
}

// The request sent to a server to start from the snapshot.
type SnapshotRequest struct {
	LeaderName string
	LastIndex  uint64
	LastTerm   uint64
}

// The response returned if the follower entered snapshot state
type SnapshotResponse struct {
	Success bool `json:"success"`
}

// save writes the snapshot to file.
func (ss *Snapshot) save() error {
	// Open the file for writing.
	file, err := os.OpenFile(ss.Path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	// Serialize to JSON.
	b, err := json.Marshal(ss)
	if err != nil {
		return err
	}

	// Generate checksum and write it to disk.
	checksum := crc32.ChecksumIEEE(b)
	if _, err = fmt.Fprintf(file, "%08x\n", checksum); err != nil {
		return err
	}

	// Write the snapshot to disk.
	if _, err = file.Write(b); err != nil {
		return err
	}

	// Ensure that the snapshot has been flushed to disk before continuing.
	if err := file.Sync(); err != nil {
		return err
	}

	return nil
}

// remove deletes the snapshot file.
func (ss *Snapshot) remove() error {
	if err := os.Remove(ss.Path); err != nil {
		return err
	}
	return nil
}

// Creates a new Snapshot request.
func newSnapshotRecoveryRequest(leaderName string, snapshot *Snapshot) *SnapshotRecoveryRequest {
	return &SnapshotRecoveryRequest{
		LeaderName: leaderName,
		LastIndex:  snapshot.LastIndex,
		LastTerm:   snapshot.LastTerm,
		Peers:      snapshot.Peers,
		State:      snapshot.State,
	}
}

// Encodes the SnapshotRecoveryRequest to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (req *SnapshotRecoveryRequest) Encode(w io.Writer) (int, error) {

	protoPeers := make([]*protobuf.SnapshotRecoveryRequest_Peer, len(req.Peers))

	for i, peer := range req.Peers {
		protoPeers[i] = &protobuf.SnapshotRecoveryRequest_Peer{
			Name:             proto.String(peer.Name),
			ConnectionString: proto.String(peer.ConnectionString),
		}
	}

	pb := &protobuf.SnapshotRecoveryRequest{
		LeaderName: proto.String(req.LeaderName),
		LastIndex:  proto.Uint64(req.LastIndex),
		LastTerm:   proto.Uint64(req.LastTerm),
		Peers:      protoPeers,
		State:      req.State,
	}
	p, err := proto.Marshal(pb)
	if err != nil {
		return -1, err
	}

	return w.Write(p)
}

// Decodes the SnapshotRecoveryRequest from a buffer. Returns the number of bytes read and
// any error that occurs.
func (req *SnapshotRecoveryRequest) Decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return 0, err
	}

	totalBytes := len(data)

	pb := &protobuf.SnapshotRecoveryRequest{}
	if err = proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	req.LeaderName = pb.GetLeaderName()
	req.LastIndex = pb.GetLastIndex()
	req.LastTerm = pb.GetLastTerm()
	req.State = pb.GetState()

	req.Peers = make([]*Peer, len(pb.Peers))

	for i, peer := range pb.Peers {
		req.Peers[i] = &Peer{
			Name:             peer.GetName(),
			ConnectionString: peer.GetConnectionString(),
		}
	}

	return totalBytes, nil
}

// Creates a new Snapshot response.
func newSnapshotRecoveryResponse(term uint64, success bool, commitIndex uint64) *SnapshotRecoveryResponse {
	return &SnapshotRecoveryResponse{
		Term:        term,
		Success:     success,
		CommitIndex: commitIndex,
	}
}

// Encode writes the response to a writer.
// Returns the number of bytes written and any error that occurs.
func (req *SnapshotRecoveryResponse) Encode(w io.Writer) (int, error) {
	pb := &protobuf.SnapshotRecoveryResponse{
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

// Creates a new Snapshot request.
func newSnapshotRequest(leaderName string, snapshot *Snapshot) *SnapshotRequest {
	return &SnapshotRequest{
		LeaderName: leaderName,
		LastIndex:  snapshot.LastIndex,
		LastTerm:   snapshot.LastTerm,
	}
}

// Encodes the SnapshotRequest to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (req *SnapshotRequest) Encode(w io.Writer) (int, error) {
	pb := &protobuf.SnapshotRequest{
		LeaderName: proto.String(req.LeaderName),
		LastIndex:  proto.Uint64(req.LastIndex),
		LastTerm:   proto.Uint64(req.LastTerm),
	}
	p, err := proto.Marshal(pb)
	if err != nil {
		return -1, err
	}

	return w.Write(p)
}

// Decodes the SnapshotRequest from a buffer. Returns the number of bytes read and
// any error that occurs.
func (req *SnapshotRequest) Decode(r io.Reader) (int, error) {
	data, err := ioutil.ReadAll(r)

	if err != nil {
		return 0, err
	}

	totalBytes := len(data)

	pb := &protobuf.SnapshotRequest{}

	if err := proto.Unmarshal(data, pb); err != nil {
		return -1, err
	}

	req.LeaderName = pb.GetLeaderName()
	req.LastIndex = pb.GetLastIndex()
	req.LastTerm = pb.GetLastTerm()

	return totalBytes, nil
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
