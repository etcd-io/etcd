package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
	"github.com/gorilla/mux"
)

// Get all the current logs
func (ps *PeerServer) GetLogHttpHandler(w http.ResponseWriter, req *http.Request) {
	log.Debugf("[recv] GET %s/log", ps.url)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ps.raftServer.LogEntries())
}

// Response to vote request
func (ps *PeerServer) VoteHttpHandler(w http.ResponseWriter, req *http.Request) {
	rvreq := &raft.RequestVoteRequest{}
	err := decodeJsonRequest(req, rvreq)
	if err == nil {
		log.Debugf("[recv] POST %s/vote [%s]", ps.url, rvreq.CandidateName)
		if resp := ps.raftServer.RequestVote(rvreq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	log.Warnf("[vote] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// Response to append entries request
func (ps *PeerServer) AppendEntriesHttpHandler(w http.ResponseWriter, req *http.Request) {
	aereq := &raft.AppendEntriesRequest{}
	err := decodeJsonRequest(req, aereq)

	if err == nil {
		log.Debugf("[recv] POST %s/log/append [%d]", ps.url, len(aereq.Entries))

		ps.serverStats.RecvAppendReq(aereq.LeaderName, int(req.ContentLength))

		if resp := ps.raftServer.AppendEntries(aereq); resp != nil {
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
func (ps *PeerServer) SnapshotHttpHandler(w http.ResponseWriter, req *http.Request) {
	aereq := &raft.SnapshotRequest{}
	err := decodeJsonRequest(req, aereq)
	if err == nil {
		log.Debugf("[recv] POST %s/snapshot/ ", ps.url)
		if resp := ps.raftServer.RequestSnapshot(aereq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	log.Warnf("[Snapshot] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// Response to recover from snapshot request
func (ps *PeerServer) SnapshotRecoveryHttpHandler(w http.ResponseWriter, req *http.Request) {
	aereq := &raft.SnapshotRecoveryRequest{}
	err := decodeJsonRequest(req, aereq)
	if err == nil {
		log.Debugf("[recv] POST %s/snapshotRecovery/ ", ps.url)
		if resp := ps.raftServer.SnapshotRecoveryRequest(aereq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	log.Warnf("[Snapshot] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// Get the port that listening for etcd connecting of the server
func (ps *PeerServer) EtcdURLHttpHandler(w http.ResponseWriter, req *http.Request) {
	log.Debugf("[recv] Get %s/etcdURL/ ", ps.url)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(ps.server.URL()))
}

// Response to the join request
func (ps *PeerServer) JoinHttpHandler(w http.ResponseWriter, req *http.Request) {
	command := &JoinCommand{}

	// Write CORS header.
	if ps.server.OriginAllowed("*") {
		w.Header().Add("Access-Control-Allow-Origin", "*")
	} else if ps.server.OriginAllowed(req.Header.Get("Origin")) {
		w.Header().Add("Access-Control-Allow-Origin", req.Header.Get("Origin"))
	}

	err := decodeJsonRequest(req, command)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Debugf("Receive Join Request from %s", command.Name)
	err = ps.server.Dispatch(command, w, req)

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
func (ps *PeerServer) RemoveHttpHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != "DELETE" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(req)
	command := &RemoveCommand{
		Name: vars["name"],
	}

	log.Debugf("[recv] Remove Request [%s]", command.Name)

	ps.server.Dispatch(command, w, req)
}

// Response to the name request
func (ps *PeerServer) NameHttpHandler(w http.ResponseWriter, req *http.Request) {
	log.Debugf("[recv] Get %s/name/ ", ps.url)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(ps.name))
}

// Response to the name request
func (ps *PeerServer) VersionHttpHandler(w http.ResponseWriter, req *http.Request) {
	log.Debugf("[recv] Get %s/version/ ", ps.url)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strconv.Itoa(ps.store.Version())))
}

// Checks whether a given version is supported.
func (ps *PeerServer) VersionCheckHttpHandler(w http.ResponseWriter, req *http.Request) {
	log.Debugf("[recv] Get %s%s ", ps.url, req.URL.Path)
	vars := mux.Vars(req)
	version, _ := strconv.Atoi(vars["version"])
	if version >= store.MinVersion() && version <= store.MaxVersion() {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

// Upgrades the current store version to the next version.
func (ps *PeerServer) UpgradeHttpHandler(w http.ResponseWriter, req *http.Request) {
	log.Debugf("[recv] Get %s/version", ps.url)

	// Check if upgrade is possible for all nodes.
	if err := ps.Upgradable(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create an upgrade command from the current version.
	c := ps.store.CommandFactory().CreateUpgradeCommand()
	if err := ps.server.Dispatch(c, w, req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
