package server

import (
	"encoding/json"
	"net/http"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/go-raft"
)

// Get all the current logs
func (s *PeerServer) GetLogHttpHandler(w http.ResponseWriter, req *http.Request) {
	log.Debugf("[recv] GET %s/log", s.url)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(s.LogEntries())
}

// Response to vote request
func (s *PeerServer) VoteHttpHandler(w http.ResponseWriter, req *http.Request) {
	rvreq := &raft.RequestVoteRequest{}
	err := decodeJsonRequest(req, rvreq)
	if err == nil {
		log.Debugf("[recv] POST %s/vote [%s]", s.url, rvreq.CandidateName)
		if resp := s.RequestVote(rvreq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	log.Warnf("[vote] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// Response to append entries request
func (s *PeerServer) AppendEntriesHttpHandler(w http.ResponseWriter, req *http.Request) {
	aereq := &raft.AppendEntriesRequest{}
	err := decodeJsonRequest(req, aereq)

	if err == nil {
		log.Debugf("[recv] POST %s/log/append [%d]", s.url, len(aereq.Entries))

		s.serverStats.RecvAppendReq(aereq.LeaderName, int(req.ContentLength))

		if resp := s.AppendEntries(aereq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			if !resp.Success {
				log.Debugf("[Append Entry] Step back")
			}
			return
		}
	}
	log.Warnf("[Append Entry] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// Response to recover from snapshot request
func (s *PeerServer) SnapshotHttpHandler(w http.ResponseWriter, req *http.Request) {
	aereq := &raft.SnapshotRequest{}
	err := decodeJsonRequest(req, aereq)
	if err == nil {
		log.Debugf("[recv] POST %s/snapshot/ ", s.url)
		if resp := s.RequestSnapshot(aereq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	log.Warnf("[Snapshot] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// Response to recover from snapshot request
func (s *PeerServer) SnapshotRecoveryHttpHandler(w http.ResponseWriter, req *http.Request) {
	aereq := &raft.SnapshotRecoveryRequest{}
	err := decodeJsonRequest(req, aereq)
	if err == nil {
		log.Debugf("[recv] POST %s/snapshotRecovery/ ", s.url)
		if resp := s.SnapshotRecoveryRequest(aereq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	log.Warnf("[Snapshot] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// Get the port that listening for etcd connecting of the server
func (s *PeerServer) EtcdURLHttpHandler(w http.ResponseWriter, req *http.Request) {
	log.Debugf("[recv] Get %s/etcdURL/ ", s.url)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(s.server.URL()))
}

// Response to the join request
func (s *PeerServer) JoinHttpHandler(w http.ResponseWriter, req *http.Request) {
	command := &JoinCommand{}

	// Write CORS header.
	if s.server.OriginAllowed("*") {
		w.Header().Add("Access-Control-Allow-Origin", "*")
	} else if s.server.OriginAllowed(req.Header.Get("Origin")) {
		w.Header().Add("Access-Control-Allow-Origin", req.Header.Get("Origin"))
	}

	err := decodeJsonRequest(req, command)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Debugf("Receive Join Request from %s", command.Name)
	err = s.dispatch(command, w, req)

	// Return status.
	if err != nil {
		if etcdErr, ok := err.(*etcdErr.Error); ok {
			log.Debug("Return error: ", (*etcdErr).Error())
			etcdErr.Write(w)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// Response to remove request
func (s *PeerServer) RemoveHttpHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != "DELETE" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	nodeName := req.URL.Path[len("/remove/"):]
	command := &RemoveCommand{
		Name: nodeName,
	}

	log.Debugf("[recv] Remove Request [%s]", command.Name)

	s.dispatch(command, w, req)
}

// Response to the name request
func (s *PeerServer) NameHttpHandler(w http.ResponseWriter, req *http.Request) {
	log.Debugf("[recv] Get %s/name/ ", s.url)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(s.name))
}

// Response to the name request
func (s *PeerServer) RaftVersionHttpHandler(w http.ResponseWriter, req *http.Request) {
	log.Debugf("[recv] Get %s/version/ ", s.url)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(PeerVersion))
}
