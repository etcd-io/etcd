package server

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	uhttp "github.com/coreos/etcd/pkg/http"
	"github.com/coreos/etcd/store"
)

const uninitedSyncClusterInterval = 5

type StandbyServerConfig struct {
	Name       string
	PeerScheme string
	PeerURL    string
	ClientURL  string
}

type StandbyServer struct {
	Config StandbyServerConfig
	client *Client

	cluster             []*machineMessage
	syncClusterInterval int
	joinIndex           uint64

	removeNotify chan bool
	started      bool
	closeChan    chan bool
	routineGroup sync.WaitGroup

	sync.Mutex
}

func NewStandbyServer(config StandbyServerConfig, client *Client) *StandbyServer {
	return &StandbyServer{
		Config:              config,
		client:              client,
		syncClusterInterval: uninitedSyncClusterInterval,
	}
}

func (s *StandbyServer) Start() {
	s.Lock()
	defer s.Unlock()
	if s.started {
		return
	}
	s.started = true

	s.removeNotify = make(chan bool)
	s.closeChan = make(chan bool)

	s.routineGroup.Add(1)
	go func() {
		defer s.routineGroup.Done()
		s.monitorCluster()
	}()
}

// Stop stops the server gracefully.
func (s *StandbyServer) Stop() {
	s.Lock()
	defer s.Unlock()
	if !s.started {
		return
	}
	s.started = false

	close(s.closeChan)
	s.routineGroup.Wait()
}

// remove stops the server in standby mode.
// It is called to stop the server because it has joined the cluster successfully.
func (s *StandbyServer) asyncRemove() {
	s.Lock()
	if !s.started {
		s.Unlock()
		return
	}
	s.started = false

	close(s.closeChan)
	go func() {
		defer s.Unlock()
		s.routineGroup.Wait()
		close(s.removeNotify)
	}()
}

// RemoveNotify notifies the server is removed from standby mode and ready
// for peer mode. It should have joined the cluster successfully.
func (s *StandbyServer) RemoveNotify() <-chan bool {
	return s.removeNotify
}

func (s *StandbyServer) ClientHTTPHandler() http.Handler {
	return http.HandlerFunc(s.redirectRequests)
}

func (s *StandbyServer) Cluster() []string {
	peerURLs := make([]string, 0)
	for _, peer := range s.cluster {
		peerURLs = append(peerURLs, peer.PeerURL)
	}
	return peerURLs
}

func (s *StandbyServer) ClusterSize() int {
	return len(s.cluster)
}

func (s *StandbyServer) setCluster(cluster []*machineMessage) {
	s.cluster = cluster
}

func (s *StandbyServer) SyncCluster(peers []string) error {
	for i, url := range peers {
		peers[i] = s.fullPeerURL(url)
	}

	if err := s.syncCluster(peers); err != nil {
		log.Infof("fail syncing cluster(%v): %v", s.Cluster(), err)
		return err
	}

	log.Infof("set cluster(%v) for standby server", s.Cluster())
	return nil
}

func (s *StandbyServer) SetSyncClusterInterval(interval int) {
	s.syncClusterInterval = interval
}

func (s *StandbyServer) ClusterLeader() *machineMessage {
	if len(s.cluster) == 0 {
		return nil
	}
	return s.cluster[0]
}

func (s *StandbyServer) JoinIndex() uint64 {
	return s.joinIndex
}

func (s *StandbyServer) redirectRequests(w http.ResponseWriter, r *http.Request) {
	leader := s.ClusterLeader()
	if leader == nil {
		w.Header().Set("Content-Type", "application/json")
		etcdErr.NewError(etcdErr.EcodeStandbyInternal, "", 0).Write(w)
		return
	}
	uhttp.Redirect(leader.ClientURL, w, r)
}

func (s *StandbyServer) monitorCluster() {
	for {
		timer := time.NewTimer(time.Duration(s.syncClusterInterval) * time.Second)
		defer timer.Stop()
		select {
		case <-s.closeChan:
			return
		case <-timer.C:
		}

		if err := s.syncCluster(nil); err != nil {
			log.Warnf("fail syncing cluster(%v): %v", s.Cluster(), err)
			continue
		}

		leader := s.ClusterLeader()
		if leader == nil {
			log.Warnf("fail getting leader from cluster(%v)", s.Cluster())
			continue
		}

		if err := s.join(leader.PeerURL); err != nil {
			log.Debugf("fail joining through leader %v: %v", leader, err)
			continue
		}

		log.Infof("join through leader %v", leader.PeerURL)
		s.asyncRemove()
		return
	}
}

func (s *StandbyServer) syncCluster(peerURLs []string) error {
	peerURLs = append(s.Cluster(), peerURLs...)

	for _, peerURL := range peerURLs {
		// Fetch current peer list
		machines, err := s.client.GetMachines(peerURL)
		if err != nil {
			log.Debugf("fail getting machine messages from %v", peerURL)
			continue
		}

		config, err := s.client.GetClusterConfig(peerURL)
		if err != nil {
			log.Debugf("fail getting cluster config from %v", peerURL)
			continue
		}

		s.setCluster(machines)
		s.SetSyncClusterInterval(config.SyncClusterInterval)
		return nil
	}
	return fmt.Errorf("unreachable cluster")
}

func (s *StandbyServer) join(peer string) error {
	// Our version must match the leaders version
	version, err := s.client.GetVersion(peer)
	if err != nil {
		log.Debugf("fail checking join version")
		return err
	}
	if version < store.MinVersion() || version > store.MaxVersion() {
		log.Debugf("fail passing version compatibility(%d-%d) using %d", store.MinVersion(), store.MaxVersion(), version)
		return fmt.Errorf("incompatible version")
	}

	// Fetch cluster config to see whether exists some place.
	clusterConfig, err := s.client.GetClusterConfig(peer)
	if err != nil {
		log.Debugf("fail getting cluster config")
		return err
	}
	if clusterConfig.ActiveSize <= len(s.Cluster()) {
		log.Debugf("stop joining because the cluster is full with %d nodes", len(s.Cluster()))
		return fmt.Errorf("out of quota")
	}

	joinResp, err := s.client.AddMachine(peer,
		&JoinCommandV2{
			MinVersion: store.MinVersion(),
			MaxVersion: store.MaxVersion(),
			Name:       s.Config.Name,
			PeerURL:    s.Config.PeerURL,
			ClientURL:  s.Config.ClientURL,
		})
	if err != nil {
		log.Debugf("fail on join request")
		return err
	}
	s.joinIndex = joinResp.CommitIndex

	return nil
}

func (s *StandbyServer) fullPeerURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		log.Warnf("fail parsing url %v", u)
		return urlStr
	}
	u.Scheme = s.Config.PeerScheme
	return u.String()
}
