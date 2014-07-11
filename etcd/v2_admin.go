package etcd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coreos/etcd/store"
)

const (
	stateFollower  = "follower"
	stateCandidate = "candidate"
	stateLeader    = "leader"
)

// machineMessage represents information about a peer or standby in the registry.
type machineMessage struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	ClientURL string `json:"clientURL"`
	PeerURL   string `json:"peerURL"`
}

type context struct {
	MinVersion int    `json:"minVersion"`
	MaxVersion int    `json:"maxVersion"`
	ClientURL  string `json:"clientURL"`
	PeerURL    string `json:"peerURL"`
}

func (s *Server) serveAdminConfig(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "GET":
	case "PUT":
		if !s.node.IsLeader() {
			return s.redirect(w, r, s.node.Leader())
		}
		c := s.ClusterConfig()
		if err := json.NewDecoder(r.Body).Decode(c); err != nil {
			return err
		}
		c.Sanitize()
		b, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if _, err := s.Set(v2configKVPrefix, false, string(b), store.Permanent); err != nil {
			return err
		}
	default:
		return allow(w, "GET", "PUT")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.ClusterConfig())
	return nil
}

func (s *Server) serveAdminMachines(w http.ResponseWriter, r *http.Request) error {
	name := strings.TrimPrefix(r.URL.Path, v2adminMachinesPrefix)
	switch r.Method {
	case "GET":
		var info interface{}
		var err error
		if name != "" {
			info, err = s.someMachineMessage(name)
		} else {
			info, err = s.allMachineMessages()
		}
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	case "PUT":
		if !s.node.IsLeader() {
			return s.redirect(w, r, s.node.Leader())
		}
		id, err := strconv.ParseInt(name, 0, 64)
		if err != nil {
			return err
		}
		info := &context{}
		if err := json.NewDecoder(r.Body).Decode(info); err != nil {
			return err
		}
		return s.Add(id, info.PeerURL, info.ClientURL)
	case "DELETE":
		if !s.node.IsLeader() {
			return s.redirect(w, r, s.node.Leader())
		}
		id, err := strconv.ParseInt(name, 0, 64)
		if err != nil {
			return err
		}
		return s.Remove(id)
	default:
		return allow(w, "GET", "PUT", "DELETE")
	}
	return nil
}

// someMachineMessage return machine message of specified name.
func (s *Server) someMachineMessage(name string) (*machineMessage, error) {
	p := filepath.Join(v2machineKVPrefix, name)
	e, err := s.Get(p, false, false)
	if err != nil {
		return nil, err
	}
	lead := fmt.Sprint(s.node.Leader())
	return newMachineMessage(e.Node, lead), nil
}

func (s *Server) allMachineMessages() ([]*machineMessage, error) {
	e, err := s.Get(v2machineKVPrefix, false, false)
	if err != nil {
		return nil, err
	}
	lead := fmt.Sprint(s.node.Leader())
	ms := make([]*machineMessage, len(e.Node.Nodes))
	for i, n := range e.Node.Nodes {
		ms[i] = newMachineMessage(n, lead)
	}
	return ms, nil
}

func newMachineMessage(n *store.NodeExtern, lead string) *machineMessage {
	_, name := filepath.Split(n.Key)
	q, err := url.ParseQuery(*n.Value)
	if err != nil {
		panic("fail to parse the info for machine " + name)
	}
	m := &machineMessage{
		Name:      name,
		State:     stateFollower,
		ClientURL: q["etcd"][0],
		PeerURL:   q["raft"][0],
	}
	if name == lead {
		m.State = stateLeader
	}
	return m
}
