package etcd

import (
	"fmt"
	"net/http"
	"strconv"
)

func (s *Server) serveRedirect(w http.ResponseWriter, r *http.Request) error {
	if s.leader == noneId {
		return fmt.Errorf("no leader in the cluster")
	}
	redirectAddr, err := s.buildRedirectURL(s.leaderAddr, r.URL)
	if err != nil {
		return err
	}
	http.Redirect(w, r, redirectAddr, http.StatusTemporaryRedirect)
	return nil
}

func (s *Server) syncCluster() error {
	for node := range s.nodes {
		machines, err := s.client.GetMachines(node)
		if err != nil {
			continue
		}
		config, err := s.client.GetClusterConfig(node)
		if err != nil {
			continue
		}
		s.nodes = make(map[string]bool)
		for _, machine := range machines {
			s.nodes[machine.PeerURL] = true
			if machine.State == stateLeader {
				id, err := strconv.ParseInt(machine.Name, 0, 64)
				if err != nil {
					return err
				}
				s.leader = id
				s.leaderAddr = machine.PeerURL
			}
		}
		s.clusterConf = config
		return nil
	}
	return fmt.Errorf("unreachable cluster")
}
