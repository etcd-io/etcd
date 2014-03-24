package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"

	"github.com/coreos/etcd/discovery"
	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/metrics"
	"github.com/coreos/etcd/store"
)

const (
	// ThresholdMonitorTimeout is the time between log notifications that the
	// Raft heartbeat is too close to the election timeout.
	ThresholdMonitorTimeout = 5 * time.Second

	// ActiveMonitorTimeout is the time between checks on the active size of
	// the cluster. If the active size is different than the actual size then
	// etcd attempts to promote/demote to bring it to the correct number.
	ActiveMonitorTimeout = 1 * time.Second

	// PeerActivityMonitorTimeout is the time between checks for dead nodes in
	// the cluster.
	PeerActivityMonitorTimeout = 1 * time.Second
)

const (
	peerModeFlag  = 0
	proxyModeFlag = 1
)

type PeerServerConfig struct {
	Name          string
	Scheme        string
	URL           string
	SnapshotCount int
	RetryTimes    int
	RetryInterval float64
}

type PeerServer struct {
	Config         PeerServerConfig
	clusterConfig  *ClusterConfig
	raftServer     raft.Server
	server         *Server
	joinIndex      uint64
	followersStats *raftFollowersStats
	serverStats    *raftServerStats
	registry       *Registry
	store          store.Store
	snapConf       *snapshotConf
	mode           Mode

	closeChan            chan bool
	timeoutThresholdChan chan interface{}

	proxyPeerURL   string
	proxyClientURL string

	metrics *metrics.Bucket
}

// TODO: find a good policy to do snapshot
type snapshotConf struct {
	// Etcd will check if snapshot is need every checkingInterval
	checkingInterval time.Duration

	// The index when the last snapshot happened
	lastIndex uint64

	// If the incremental number of index since the last snapshot
	// exceeds the snapshot Threshold, etcd will do a snapshot
	snapshotThr uint64
}

func NewPeerServer(psConfig PeerServerConfig, registry *Registry, store store.Store, mb *metrics.Bucket, followersStats *raftFollowersStats, serverStats *raftServerStats) *PeerServer {
	s := &PeerServer{
		Config:         psConfig,
		clusterConfig:  NewClusterConfig(),
		registry:       registry,
		store:          store,
		followersStats: followersStats,
		serverStats:    serverStats,

		timeoutThresholdChan: make(chan interface{}, 1),

		metrics: mb,
	}

	return s
}

func (s *PeerServer) SetRaftServer(raftServer raft.Server) {
	s.snapConf = &snapshotConf{
		checkingInterval: time.Second * 3,
		// this is not accurate, we will update raft to provide an api
		lastIndex:   raftServer.CommitIndex(),
		snapshotThr: uint64(s.Config.SnapshotCount),
	}

	raftServer.AddEventListener(raft.StateChangeEventType, s.raftEventLogger)
	raftServer.AddEventListener(raft.LeaderChangeEventType, s.raftEventLogger)
	raftServer.AddEventListener(raft.TermChangeEventType, s.raftEventLogger)
	raftServer.AddEventListener(raft.AddPeerEventType, s.raftEventLogger)
	raftServer.AddEventListener(raft.RemovePeerEventType, s.raftEventLogger)
	raftServer.AddEventListener(raft.HeartbeatIntervalEventType, s.raftEventLogger)
	raftServer.AddEventListener(raft.ElectionTimeoutThresholdEventType, s.raftEventLogger)

	raftServer.AddEventListener(raft.HeartbeatEventType, s.recordMetricEvent)

	s.raftServer = raftServer
}

// Mode retrieves the current mode of the server.
func (s *PeerServer) Mode() Mode {
	return s.mode
}

// SetMode updates the current mode of the server.
// Switching to a peer mode will start the Raft server.
// Switching to a proxy mode will stop the Raft server.
func (s *PeerServer) setMode(mode Mode) {
	s.mode = mode

	switch mode {
	case PeerMode:
		if !s.raftServer.Running() {
			s.raftServer.Start()
		}
	case ProxyMode:
		if s.raftServer.Running() {
			s.raftServer.Stop()
		}
	}
}

// ClusterConfig retrieves the current cluster configuration.
func (s *PeerServer) ClusterConfig() *ClusterConfig {
	return s.clusterConfig
}

// SetClusterConfig updates the current cluster configuration.
// Adjusting the active size will cause the PeerServer to demote peers or
// promote proxies to match the new size.
func (s *PeerServer) SetClusterConfig(c *ClusterConfig) {
	// Set minimums.
	if c.ActiveSize < MinActiveSize {
		c.ActiveSize = MinActiveSize
	}
	if c.PromoteDelay < MinPromoteDelay {
		c.PromoteDelay = MinPromoteDelay
	}

	s.clusterConfig = c
}

// Helper function to do discovery and return results in expected format
func (s *PeerServer) handleDiscovery(discoverURL string) (peers []string, err error) {
	peers, err = discovery.Do(discoverURL, s.Config.Name, s.Config.URL)

	// Warn about errors coming from discovery, this isn't fatal
	// since the user might have provided a peer list elsewhere,
	// or there is some log in data dir.
	if err != nil {
		log.Warnf("Discovery encountered an error: %v", err)
		return
	}

	for i := range peers {
		// Strip the scheme off of the peer if it has one
		// TODO(bp): clean this up!
		purl, err := url.Parse(peers[i])
		if err == nil {
			peers[i] = purl.Host
		}
	}

	log.Infof("Discovery fetched back peer list: %v", peers)

	return
}

// Try all possible ways to find clusters to join
// Include -discovery, -peers and log data in -data-dir
//
// Peer discovery follows this order:
// 1. -discovery
// 2. -peers
// 3. previous peers in -data-dir
// RaftServer should be started as late as possible. Current implementation
// to start it is not that good, and will be refactored in #627.
func (s *PeerServer) findCluster(discoverURL string, peers []string) {
	// Attempt cluster discovery
	toDiscover := discoverURL != ""
	if toDiscover {
		discoverPeers, discoverErr := s.handleDiscovery(discoverURL)
		// It is registered in discover url
		if discoverErr == nil {
			// start as a leader in a new cluster
			if len(discoverPeers) == 0 {
				log.Debug("This peer is starting a brand new cluster based on discover URL.")
				s.startAsLeader()
			} else {
				s.startAsFollower(discoverPeers)
			}
			return
		}
	}

	hasPeerList := len(peers) > 0
	// if there is log in data dir, append previous peers to peers in config
	// to find cluster
	prevPeers := s.registry.PeerURLs(s.raftServer.Leader(), s.Config.Name)
	for i := 0; i < len(prevPeers); i++ {
		u, err := url.Parse(prevPeers[i])
		if err != nil {
			log.Debug("rejoin cannot parse url: ", err)
		}
		prevPeers[i] = u.Host
	}
	peers = append(peers, prevPeers...)

	// Remove its own peer address from the peer list to join
	u, err := url.Parse(s.Config.URL)
	if err != nil {
		log.Fatalf("cannot parse peer address %v: %v", s.Config.URL, err)
	}
	filteredPeers := make([]string, 0)
	for _, v := range peers {
		if v != u.Host {
			filteredPeers = append(filteredPeers, v)
		}
	}
	peers = filteredPeers

	// if there is backup peer lists, use it to find cluster
	if len(peers) > 0 {
		ok := s.joinCluster(peers)
		if !ok {
			log.Warn("No living peers are found!")
		} else {
			s.raftServer.Start()
			log.Debugf("%s restart as a follower based on peers[%v]", s.Config.Name)
			return
		}
	}

	if !s.raftServer.IsLogEmpty() {
		log.Debug("Entire cluster is down! %v will restart the cluster.", s.Config.Name)
		s.raftServer.Start()
		return
	}

	if toDiscover {
		log.Fatalf("Discovery failed, no available peers in backup list, and no log data")
	}

	if hasPeerList {
		log.Fatalf("No available peers in backup list, and no log data")
	}

	log.Infof("This peer is starting a brand new cluster now.")
	s.startAsLeader()
}

// Start the raft server
func (s *PeerServer) Start(snapshot bool, discoverURL string, peers []string) error {
	// LoadSnapshot
	if snapshot {
		err := s.raftServer.LoadSnapshot()

		if err == nil {
			log.Debugf("%s finished load snapshot", s.Config.Name)
		} else {
			log.Debug(err)
		}
	}

	s.raftServer.Init()

	s.findCluster(discoverURL, peers)

	s.closeChan = make(chan bool)

	go s.monitorSync()
	go s.monitorTimeoutThreshold(s.closeChan)
	go s.monitorActiveSize(s.closeChan)
	go s.monitorPeerActivity(s.closeChan)

	// open the snapshot
	if snapshot {
		go s.monitorSnapshot()
	}

	return nil
}

func (s *PeerServer) Stop() {
	if s.closeChan != nil {
		close(s.closeChan)
		s.closeChan = nil
	}
	s.raftServer.Stop()
}

func (s *PeerServer) HTTPHandler() http.Handler {
	router := mux.NewRouter()

	// internal commands
	router.HandleFunc("/name", s.NameHttpHandler)
	router.HandleFunc("/version", s.VersionHttpHandler)
	router.HandleFunc("/version/{version:[0-9]+}/check", s.VersionCheckHttpHandler)
	router.HandleFunc("/upgrade", s.UpgradeHttpHandler)
	router.HandleFunc("/join", s.JoinHttpHandler)
	router.HandleFunc("/promote", s.PromoteHttpHandler).Methods("POST")
	router.HandleFunc("/remove/{name:.+}", s.RemoveHttpHandler)
	router.HandleFunc("/vote", s.VoteHttpHandler)
	router.HandleFunc("/log", s.GetLogHttpHandler)
	router.HandleFunc("/log/append", s.AppendEntriesHttpHandler)
	router.HandleFunc("/snapshot", s.SnapshotHttpHandler)
	router.HandleFunc("/snapshotRecovery", s.SnapshotRecoveryHttpHandler)
	router.HandleFunc("/etcdURL", s.EtcdURLHttpHandler)

	router.HandleFunc("/v2/admin/config", s.getClusterConfigHttpHandler).Methods("GET")
	router.HandleFunc("/v2/admin/config", s.setClusterConfigHttpHandler).Methods("PUT")
	router.HandleFunc("/v2/admin/machines", s.getMachinesHttpHandler).Methods("GET")
	router.HandleFunc("/v2/admin/machines/{name}", s.getMachineHttpHandler).Methods("GET")
	router.HandleFunc("/v2/admin/machines/{name}", s.addMachineHttpHandler).Methods("PUT")
	router.HandleFunc("/v2/admin/machines/{name}", s.removeMachineHttpHandler).Methods("DELETE")

	return router
}

// Retrieves the underlying Raft server.
func (s *PeerServer) RaftServer() raft.Server {
	return s.raftServer
}

// Associates the client server with the peer server.
func (s *PeerServer) SetServer(server *Server) {
	s.server = server
}

func (s *PeerServer) startAsLeader() {
	s.raftServer.Start()
	// leader need to join self as a peer
	for {
		c := &JoinCommandV1{
			MinVersion: store.MinVersion(),
			MaxVersion: store.MaxVersion(),
			Name:       s.raftServer.Name(),
			RaftURL:    s.Config.URL,
			EtcdURL:    s.server.URL(),
		}
		_, err := s.raftServer.Do(c)
		if err == nil {
			break
		}
	}
	log.Debugf("%s start as a leader", s.Config.Name)
}

func (s *PeerServer) startAsFollower(cluster []string) {
	// start as a follower in a existing cluster
	for i := 0; i < s.Config.RetryTimes; i++ {
		ok := s.joinCluster(cluster)
		if ok {
			s.raftServer.Start()
			return
		}
		log.Warnf("%v is unable to join the cluster using any of the peers %v at %dth time. Retrying in %.1f seconds", s.Config.Name, cluster, i, s.Config.RetryInterval)
		time.Sleep(time.Second * time.Duration(s.Config.RetryInterval))
	}

	log.Fatalf("Cannot join the cluster via given peers after %x retries", s.Config.RetryTimes)
}

// getVersion fetches the peer version of a cluster.
func getVersion(t *transporter, versionURL url.URL) (int, error) {
	resp, _, err := t.Get(versionURL.String())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// Parse version number.
	version, _ := strconv.Atoi(string(body))
	return version, nil
}

// Upgradable checks whether all peers in a cluster support an upgrade to the next store version.
func (s *PeerServer) Upgradable() error {
	nextVersion := s.store.Version() + 1
	for _, peerURL := range s.registry.PeerURLs(s.raftServer.Leader(), s.Config.Name) {
		u, err := url.Parse(peerURL)
		if err != nil {
			return fmt.Errorf("PeerServer: Cannot parse URL: '%s' (%s)", peerURL, err)
		}

		t, _ := s.raftServer.Transporter().(*transporter)
		checkURL := (&url.URL{Host: u.Host, Scheme: s.Config.Scheme, Path: fmt.Sprintf("/version/%d/check", nextVersion)}).String()
		resp, _, err := t.Get(checkURL)
		if err != nil {
			return fmt.Errorf("PeerServer: Cannot check version compatibility: %s", u.Host)
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("PeerServer: Version %d is not compatible with peer: %s", nextVersion, u.Host)
		}
	}

	return nil
}

func (s *PeerServer) joinCluster(cluster []string) bool {
	for _, peer := range cluster {
		if len(peer) == 0 {
			continue
		}

		err := s.joinByPeer(s.raftServer, peer, s.Config.Scheme)
		if err == nil {
			log.Debugf("%s joined the cluster via peer %s", s.Config.Name, peer)
			return true

		}

		if _, ok := err.(etcdErr.Error); ok {
			log.Fatal(err)
		}

		log.Warnf("Attempt to join via %s failed: %s", peer, err)
	}

	return false
}

// Send join requests to peer.
func (s *PeerServer) joinByPeer(server raft.Server, peer string, scheme string) error {
	// t must be ok
	t, _ := server.Transporter().(*transporter)

	// Our version must match the leaders version
	versionURL := url.URL{Host: peer, Scheme: scheme, Path: "/version"}
	version, err := getVersion(t, versionURL)
	if err != nil {
		return fmt.Errorf("Error during join version check: %v", err)
	}
	if version < store.MinVersion() || version > store.MaxVersion() {
		return fmt.Errorf("Unable to join: cluster version is %d; version compatibility is %d - %d", version, store.MinVersion(), store.MaxVersion())
	}

	var b bytes.Buffer
	c := &JoinCommandV2{
		MinVersion: store.MinVersion(),
		MaxVersion: store.MaxVersion(),
		Name:       server.Name(),
		PeerURL:    s.Config.URL,
		ClientURL:  s.server.URL(),
	}
	json.NewEncoder(&b).Encode(c)

	joinURL := url.URL{Host: peer, Scheme: scheme, Path: "/v2/admin/machines/" + server.Name()}
	log.Infof("Send Join Request to %s", joinURL.String())

	req, _ := http.NewRequest("PUT", joinURL.String(), &b)
	resp, err := t.client.Do(req)

	for {
		if err != nil {
			return fmt.Errorf("Unable to join: %v", err)
		}
		if resp != nil {
			defer resp.Body.Close()

			log.Infof("»»»» %d", resp.StatusCode)
			if resp.StatusCode == http.StatusOK {
				var msg joinMessageV2
				if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
					log.Debugf("Error reading join response: %v", err)
					return err
				}
				s.joinIndex = msg.CommitIndex
				s.setMode(msg.Mode)

				if msg.Mode == ProxyMode {
					s.proxyClientURL = resp.Header.Get("X-Leader-Client-URL")
					s.proxyPeerURL = resp.Header.Get("X-Leader-Peer-URL")
				}

				return nil
			}
			if resp.StatusCode == http.StatusTemporaryRedirect {
				address := resp.Header.Get("Location")
				log.Debugf("Send Join Request to %s", address)
				c := &JoinCommandV1{
					MinVersion: store.MinVersion(),
					MaxVersion: store.MaxVersion(),
					Name:       server.Name(),
					RaftURL:    s.Config.URL,
					EtcdURL:    s.server.URL(),
				}
				json.NewEncoder(&b).Encode(c)
				resp, _, err = t.Post(address, &b)

			} else if resp.StatusCode == http.StatusBadRequest {
				log.Debug("Reach max number peers in the cluster")
				decoder := json.NewDecoder(resp.Body)
				err := &etcdErr.Error{}
				decoder.Decode(err)
				return *err
			} else {
				return fmt.Errorf("Unable to join")
			}
		}

	}
}

func (s *PeerServer) Stats() []byte {
	s.serverStats.LeaderInfo.Uptime = time.Now().Sub(s.serverStats.LeaderInfo.startTime).String()

	// TODO: register state listener to raft to change this field
	// rather than compare the state each time Stats() is called.
	if s.RaftServer().State() == raft.Leader {
		s.serverStats.LeaderInfo.Name = s.RaftServer().Name()
	}

	queue := s.serverStats.sendRateQueue

	s.serverStats.SendingPkgRate, s.serverStats.SendingBandwidthRate = queue.Rate()

	queue = s.serverStats.recvRateQueue

	s.serverStats.RecvingPkgRate, s.serverStats.RecvingBandwidthRate = queue.Rate()

	b, _ := json.Marshal(s.serverStats)

	return b
}

func (s *PeerServer) PeerStats() []byte {
	if s.raftServer.State() == raft.Leader {
		b, _ := json.Marshal(s.followersStats)
		return b
	}
	return nil
}

// raftEventLogger converts events from the Raft server into log messages.
func (s *PeerServer) raftEventLogger(event raft.Event) {
	value := event.Value()
	prevValue := event.PrevValue()
	if value == nil {
		value = "<nil>"
	}
	if prevValue == nil {
		prevValue = "<nil>"
	}

	switch event.Type() {
	case raft.StateChangeEventType:
		log.Infof("%s: state changed from '%v' to '%v'.", s.Config.Name, prevValue, value)
	case raft.TermChangeEventType:
		log.Infof("%s: term #%v started.", s.Config.Name, value)
	case raft.LeaderChangeEventType:
		log.Infof("%s: leader changed from '%v' to '%v'.", s.Config.Name, prevValue, value)
	case raft.AddPeerEventType:
		log.Infof("%s: peer added: '%v'", s.Config.Name, value)
	case raft.RemovePeerEventType:
		log.Infof("%s: peer removed: '%v'", s.Config.Name, value)
	case raft.HeartbeatIntervalEventType:
		var name = "<unknown>"
		if peer, ok := value.(*raft.Peer); ok {
			name = peer.Name
		}
		log.Infof("%s: warning: heartbeat timed out: '%v'", s.Config.Name, name)
	case raft.ElectionTimeoutThresholdEventType:
		select {
		case s.timeoutThresholdChan <- value:
		default:
		}

	}
}

func (s *PeerServer) recordMetricEvent(event raft.Event) {
	name := fmt.Sprintf("raft.event.%s", event.Type())
	value := event.Value().(time.Duration)
	(*s.metrics).Timer(name).Update(value)
}

// logSnapshot logs about the snapshot that was taken.
func (s *PeerServer) logSnapshot(err error, currentIndex, count uint64) {
	info := fmt.Sprintf("%s: snapshot of %d events at index %d", s.Config.Name, count, currentIndex)

	if err != nil {
		log.Infof("%s attempted and failed: %v", info, err)
	} else {
		log.Infof("%s completed", info)
	}
}

func (s *PeerServer) monitorSnapshot() {
	for {
		time.Sleep(s.snapConf.checkingInterval)
		currentIndex := s.RaftServer().CommitIndex()
		count := currentIndex - s.snapConf.lastIndex
		if uint64(count) > s.snapConf.snapshotThr {
			err := s.raftServer.TakeSnapshot()
			s.logSnapshot(err, currentIndex, count)
			s.snapConf.lastIndex = currentIndex
		}
	}
}

func (s *PeerServer) monitorSync() {
	ticker := time.Tick(time.Millisecond * 500)
	for {
		select {
		case now := <-ticker:
			if s.raftServer.State() == raft.Leader {
				s.raftServer.Do(s.store.CommandFactory().CreateSyncCommand(now))
			}
		}
	}
}

// monitorTimeoutThreshold groups timeout threshold events together and prints
// them as a single log line.
func (s *PeerServer) monitorTimeoutThreshold(closeChan chan bool) {
	for {
		select {
		case value := <-s.timeoutThresholdChan:
			log.Infof("%s: warning: heartbeat near election timeout: %v", s.Config.Name, value)
		case <-closeChan:
			return
		}

		time.Sleep(ThresholdMonitorTimeout)
	}
}

// monitorActiveSize has the leader periodically check the status of cluster
// nodes and swaps them out for proxies as needed.
func (s *PeerServer) monitorActiveSize(closeChan chan bool) {
	for {
		select {
		case <-time.After(ActiveMonitorTimeout):
		case <-closeChan:
			return
		}

		// Ignore while this peer is not a leader.
		if s.raftServer.State() != raft.Leader {
			continue
		}

		// Retrieve target active size and actual active size.
		activeSize := s.ClusterConfig().ActiveSize
		peerCount := s.registry.PeerCount()
		proxies := s.registry.Proxies()
		peers := s.registry.Peers()
		if index := sort.SearchStrings(peers, s.Config.Name); index < len(peers) && peers[index] == s.Config.Name {
			peers = append(peers[:index], peers[index+1:]...)
		}

		// If we have more active nodes than we should then demote.
		if peerCount > activeSize {
			peer := peers[rand.Intn(len(peers))]
			log.Infof("%s: demoting: %v", s.Config.Name, peer)
			if _, err := s.raftServer.Do(&DemoteCommand{Name: peer}); err != nil {
				log.Infof("%s: warning: demotion error: %v", s.Config.Name, err)
			}
			continue
		}

		// If we don't have enough active nodes then try to promote a proxy.
		if peerCount < activeSize && len(proxies) > 0 {
		loop:
			for _, i := range rand.Perm(len(proxies)) {
				proxy := proxies[i]
				proxyPeerURL, _ := s.registry.ProxyPeerURL(proxy)
				log.Infof("%s: attempting to promote: %v (%s)", s.Config.Name, proxy, proxyPeerURL)

				// Notify proxy to promote itself.
				client := &http.Client{
					Transport: &http.Transport{
						DisableKeepAlives:     false,
						ResponseHeaderTimeout: ActiveMonitorTimeout,
					},
				}
				resp, err := client.Post(fmt.Sprintf("%s/promote", proxyPeerURL), "application/json", nil)
				if err != nil {
					log.Infof("%s: warning: promotion error: %v", s.Config.Name, err)
					continue
				} else if resp.StatusCode != http.StatusOK {
					log.Infof("%s: warning: promotion failure: %v", s.Config.Name, resp.StatusCode)
					continue
				}
				break loop
			}
		}
	}
}

// monitorPeerActivity has the leader periodically for dead nodes and demotes them.
func (s *PeerServer) monitorPeerActivity(closeChan chan bool) {
	for {
		select {
		case <-time.After(PeerActivityMonitorTimeout):
		case <-closeChan:
			return
		}

		// Ignore while this peer is not a leader.
		if s.raftServer.State() != raft.Leader {
			continue
		}

		// Check last activity for all peers.
		now := time.Now()
		promoteDelay := time.Duration(s.ClusterConfig().PromoteDelay) * time.Second
		peers := s.raftServer.Peers()
		for _, peer := range peers {
			// If the last response from the peer is longer than the promote delay
			// then automatically demote the peer.
			if !peer.LastActivity().IsZero() && now.Sub(peer.LastActivity()) > promoteDelay {
				log.Infof("%s: demoting node: %v; last activity %v ago", s.Config.Name, peer.Name, now.Sub(peer.LastActivity()))
				if _, err := s.raftServer.Do(&DemoteCommand{Name: peer.Name}); err != nil {
					log.Infof("%s: warning: autodemotion error: %v", s.Config.Name, err)
				}
				continue
			}
		}
	}
}

// Mode represents whether the server is an active peer or if the server is
// simply acting as a proxy.
type Mode string

const (
	// PeerMode is when the server is an active node in Raft.
	PeerMode = Mode("peer")

	// ProxyMode is when the server is an inactive, request-forwarding node.
	ProxyMode = Mode("proxy")
)
