package raft

import (
	"io"
	"io/ioutil"

	"github.com/coreos/etcd/third_party/code.google.com/p/gogoprotobuf/proto"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft/protobuf"
)

// The request sent to a server to start from the snapshot.
type SnapshotRecoveryRequest struct {
	LeaderName	string
	LastIndex	uint64
	LastTerm	uint64
	Peers		[]*Peer
	State		[]byte
}

// Creates a new Snapshot request.
func newSnapshotRecoveryRequest(leaderName string, snapshot *Snapshot) *SnapshotRecoveryRequest {
	return &SnapshotRecoveryRequest{
		LeaderName:	leaderName,
		LastIndex:	snapshot.LastIndex,
		LastTerm:	snapshot.LastTerm,
		Peers:		snapshot.Peers,
		State:		snapshot.State,
	}
}

// Encodes the SnapshotRecoveryRequest to a buffer. Returns the number of bytes
// written and any error that may have occurred.
func (req *SnapshotRecoveryRequest) Encode(w io.Writer) (int, error) {

	protoPeers := make([]*protobuf.SnapshotRecoveryRequest_Peer, len(req.Peers))

	for i, peer := range req.Peers {
		protoPeers[i] = &protobuf.SnapshotRecoveryRequest_Peer{
			Name:			proto.String(peer.Name),
			ConnectionString:	proto.String(peer.ConnectionString),
		}
	}

	pb := &protobuf.SnapshotRecoveryRequest{
		LeaderName:	proto.String(req.LeaderName),
		LastIndex:	proto.Uint64(req.LastIndex),
		LastTerm:	proto.Uint64(req.LastTerm),
		Peers:		protoPeers,
		State:		req.State,
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
			Name:			peer.GetName(),
			ConnectionString:	peer.GetConnectionString(),
		}
	}

	return totalBytes, nil
}
