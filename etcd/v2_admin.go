package etcd

import (
	"encoding/json"
	"net/http"

	"github.com/coreos/etcd/store"
)

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
