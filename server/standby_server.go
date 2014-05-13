package server

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coreos/etcd/third_party/github.com/goraft/raft"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	uhttp "github.com/coreos/etcd/pkg/http"
	"github.com/coreos/etcd/store"
)

const UninitedSyncInterval = time.Duration(5) * time.Second

type StandbyServerConfig struct {
	Name       string
	PeerScheme string
	PeerURL    string
	ClientURL  string
}

type StandbyServer struct {
	Config StandbyServerConfig
	client *Client

	cluster      []*machineMessage
	syncInterval time.Duration
	joinIndex    uint64

	removeNotify chan bool
	started      bool
	closeChan    chan bool
	routineGroup sync.WaitGroup

	sync.Mutex
}

func NewStandbyServer(config StandbyServerConfig, client *Client) *StandbyServer {
	return &StandbyServer{
		Config:       config,
		client:       client,
		syncInterval: UninitedSyncInterval,
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

func (s *StandbyServer) SetSyncInterval(second float64) {
	s.syncInterval = time.Duration(int64(second * float64(time.Second)))
}

func (s *StandbyServer) ClusterLeader() *machineMessage {
	for _, machine := range s.cluster {
		if machine.State == raft.Leader {
			return machine
		}
	}
	return nil
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
		timer := time.NewTimer(s.syncInterval)
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
		go func() {
			s.Stop()
			close(s.removeNotify)
		}()
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
		s.SetSyncInterval(config.SyncInterval)
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

	commitIndex, err := s.client.AddMachine(peer,
		&JoinCommand{
			MinVersion: store.MinVersion(),
			MaxVersion: store.MaxVersion(),
			Name:       s.Config.Name,
			RaftURL:    s.Config.PeerURL,
			EtcdURL:    s.Config.ClientURL,
		})
	if err != nil {
		log.Debugf("fail on join request")
		return err
	}
	s.joinIndex = commitIndex

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
