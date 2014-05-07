package server

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"

	"github.com/coreos/etcd/discovery"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/metrics"
	"github.com/coreos/etcd/pkg/btrfs"
	"github.com/coreos/etcd/store"
)

const (
	// ThresholdMonitorTimeout is the time between log notifications that the
	// Raft heartbeat is too close to the election timeout.
	ThresholdMonitorTimeout = 5 * time.Second

	// ActiveMonitorTimeout is the time between checks on the active size of
	// the cluster. If the active size is bigger than the actual size then
	// etcd attempts to demote to bring it to the correct number.
	ActiveMonitorTimeout = 1 * time.Second

	// PeerActivityMonitorTimeout is the time between checks for dead nodes in
	// the cluster.
	PeerActivityMonitorTimeout = 1 * time.Second

	// The location of cluster config in key space.
	ClusterConfigKey = "/_etcd/config"
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
	client         *Client
	raftServer     raft.Server
	server         *Server
	followersStats *raftFollowersStats
	serverStats    *raftServerStats
	registry       *Registry
	store          store.Store
	snapConf       *snapshotConf

	joinIndex        uint64
	isNewCluster     bool
	standbyModeInLog bool

	removeNotify         chan bool
	started              bool
	closeChan            chan bool
	routineGroup         sync.WaitGroup
	timeoutThresholdChan chan interface{}

	metrics *metrics.Bucket
	sync.Mutex
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

func NewPeerServer(psConfig PeerServerConfig, client *Client, registry *Registry, store store.Store, mb *metrics.Bucket, followersStats *raftFollowersStats, serverStats *raftServerStats) *PeerServer {
	s := &PeerServer{
		Config:         psConfig,
		client:         client,
		registry:       registry,
		store:          store,
		followersStats: followersStats,
		serverStats:    serverStats,

		timeoutThresholdChan: make(chan interface{}, 1),

		metrics: mb,
	}

	return s
}

func (s *PeerServer) SetRaftServer(raftServer raft.Server, snapshot bool) {
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
	s.standbyModeInLog = false

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

	// Set NOCOW for data directory in btrfs
	if btrfs.IsBtrfs(s.raftServer.LogPath()) {
		if err := btrfs.SetNOCOWFile(s.raftServer.LogPath()); err != nil {
			log.Warnf("Failed setting NOCOW: %v", err)
		}
	}
}

// Try all possible ways to find clusters to join
// Include log data in -data-dir, -discovery and -peers
//
// Peer discovery follows this order:
// 1. previous peers in -data-dir
// 2. -discovery
// 3. -peers
func (s *PeerServer) FindCluster(discoverURL string, peers []string) (useStandbyMode bool, possiblePeers []string, err error) {
	name := s.Config.Name
	isNewNode := s.raftServer.IsLogEmpty()
	var joinErr error

	// Try its best to find possible peers, and connect with them.
	if !isNewNode {
		// It is not allowed to join the cluster with existing peer address
		// This prevents old node joining with different name by mistake.
		if !s.checkPeerAddressNonconflict() {
			err = fmt.Errorf("%v is not allowed to join the cluster with existing URL %v", s.Config.Name, s.Config.URL)
			return
		}

		// Take old nodes into account.
		possiblePeers = s.getKnownPeers()
		// Discover registered peers.
		// TODO(yichengq): It may mess up discoverURL if this is
		// set wrong by mistake. This may need to refactor discovery
		// module. Fix it later.
		if discoverURL != "" {
			discoverPeers, _ := s.handleDiscovery(discoverURL)
			possiblePeers = append(possiblePeers, discoverPeers...)
		}
		possiblePeers = append(possiblePeers, peers...)
		possiblePeers = s.removeSelfFromList(possiblePeers)

		if s.standbyModeInLog {
			useStandbyMode = true
			return
		}

		// If there is possible peer list, use it to find cluster.
		if len(possiblePeers) > 0 {
			// TODO(yichengq): joinCluster may fail if there's no leader for
			// current cluster. It should wait if the cluster is under
			// leader election, or the node with changed IP cannot join
			// the cluster then.
			if joinErr, err = s.startAsFollower(possiblePeers, 1); joinErr != nil {
				log.Debugf("%s should work as standby for the cluster %v", name, possiblePeers)
				useStandbyMode = true
				return
			} else if err == nil {
				log.Debugf("%s joins to the previous cluster %v", name, possiblePeers)
				return
			}

			log.Warnf("%s cannot connect to previous cluster %v: %v", name, possiblePeers, err)
			err = nil
		}

		// TODO(yichengq): Think about the action that should be done
		// if it cannot connect any of the previous known node.
		log.Debugf("%s is restarting the cluster %v", name, possiblePeers)
		return
	}

	// Attempt cluster discovery
	if discoverURL != "" {
		discoverPeers, discoverErr := s.handleDiscovery(discoverURL)
		// It is registered in discover url
		if discoverErr == nil {
			// start as a leader in a new cluster
			if len(discoverPeers) == 0 {
				s.isNewCluster = true
				log.Debugf("%s is starting a new cluster via discover service", name)
				return
			}

			log.Debugf("%s is joining a cluster %v via discover service", name, discoverPeers)
			if joinErr, err = s.startAsFollower(discoverPeers, s.Config.RetryTimes); err != nil {
				log.Warnf("%s cannot connect to existing cluster %v", name, discoverPeers)
			} else if joinErr != nil {
				log.Debugf("%s should work as standby for the cluster %v", name, discoverPeers)
				useStandbyMode = true
				possiblePeers = discoverPeers
			}
			return
		}
		log.Warnf("%s failed to connect discovery service[%v]: %v", name, discoverURL, discoverErr)

		if len(peers) == 0 {
			err = fmt.Errorf("%s, the new instance, must register itself to discovery service as required", name)
			return
		}
	}

	if len(peers) > 0 {
		log.Debugf("%s is joining peers %v from -peers flag", name, peers)
		if joinErr, err = s.startAsFollower(peers, s.Config.RetryTimes); err != nil {
			log.Warnf("%s cannot connect to existing peers %v", name, peers)
		} else if joinErr != nil {
			log.Debugf("%s should work as standby for the cluster %v", name, peers)
			useStandbyMode = true
			possiblePeers = peers
		}
		return
	}

	s.isNewCluster = true
	log.Infof("%s is starting a new cluster.", s.Config.Name)
	return
}

// Start starts the raft server.
// The function assumes that join has been accepted successfully.
func (s *PeerServer) Start(snapshot bool) error {
	s.Lock()
	defer s.Unlock()
	if s.started {
		return nil
	}
	s.started = true

	s.removeNotify = make(chan bool)
	s.closeChan = make(chan bool)

	s.raftServer.Start()
	if s.isNewCluster {
		s.InitNewCluster()
		s.isNewCluster = false
	}

	s.daemon(s.monitorSync)
	s.daemon(s.monitorTimeoutThreshold)
	s.daemon(s.monitorActiveSize)
	s.daemon(s.monitorPeerActivity)

	// open the snapshot
	if snapshot {
		s.daemon(s.monitorSnapshot)
	}

	return nil
}

// Stop stops the server gracefully.
func (s *PeerServer) Stop() {
	s.Lock()
	defer s.Unlock()
	if !s.started {
		return
	}
	s.started = false

	s.triggerStop()
	s.waitStop()
}

// asyncRemove stops the server in peer mode.
// It is called to stop the server because it has been removed
// from the cluster.
func (s *PeerServer) asyncRemove() {
	s.Lock()
	if !s.started {
		s.Unlock()
		return
	}
	s.started = false

	s.triggerStop()
	go func() {
		defer s.Unlock()
		s.waitStop()
		close(s.removeNotify)
	}()
}

func (s *PeerServer) triggerStop() {
	close(s.closeChan)
	// TODO(yichengq): it should also call async stop for raft server,
	// but this functionality has not been implemented.
}

func (s *PeerServer) waitStop() {
	s.raftServer.Stop()
	s.routineGroup.Wait()
}

// RemoveNotify notifies the server is removed from peer mode due to
// removal from the cluster.
func (s *PeerServer) RemoveNotify() <-chan bool {
	return s.removeNotify
}

func (s *PeerServer) HTTPHandler() http.Handler {
	router := mux.NewRouter()

	// internal commands
	router.HandleFunc("/name", s.NameHttpHandler)
	router.HandleFunc("/version", s.VersionHttpHandler)
	router.HandleFunc("/version/{version:[0-9]+}/check", s.VersionCheckHttpHandler)
	router.HandleFunc("/upgrade", s.UpgradeHttpHandler)
	router.HandleFunc("/join", s.JoinHttpHandler)
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

func (s *PeerServer) SetJoinIndex(joinIndex uint64) {
	s.joinIndex = joinIndex
}

// ClusterConfig retrieves the current cluster configuration.
func (s *PeerServer) ClusterConfig() *ClusterConfig {
	e, err := s.store.Get(ClusterConfigKey, false, false)
	// This is useful for backward compatibility because it doesn't
	// set cluster config in older version.
	if err != nil {
		log.Debugf("failed getting cluster config key: %v", err)
		return NewClusterConfig()
	}

	var c ClusterConfig
	if err = json.Unmarshal([]byte(*e.Node.Value), &c); err != nil {
		log.Debugf("failed unmarshaling cluster config: %v", err)
		return NewClusterConfig()
	}
	return &c
}

// SetClusterConfig updates the current cluster configuration.
// Adjusting the active size will cause the PeerServer to demote peers or
// promote standbys to match the new size.
func (s *PeerServer) SetClusterConfig(c *ClusterConfig) {
	// Set minimums.
	if c.ActiveSize < MinActiveSize {
		c.ActiveSize = MinActiveSize
	}
	if c.RemoveDelay < MinRemoveDelay {
		c.RemoveDelay = MinRemoveDelay
	}
	if c.SyncClusterInterval < MinSyncClusterInterval {
		c.SyncClusterInterval = MinSyncClusterInterval
	}

	log.Debugf("set cluster config as %v", c)
	b, _ := json.Marshal(c)
	s.store.Set(ClusterConfigKey, false, string(b), store.Permanent)
}

// Retrieves the underlying Raft server.
func (s *PeerServer) RaftServer() raft.Server {
	return s.raftServer
}

// Associates the client server with the peer server.
func (s *PeerServer) SetServer(server *Server) {
	s.server = server
}

func (s *PeerServer) InitNewCluster() {
	// leader need to join self as a peer
	s.doCommand(&JoinCommandV2{
		MinVersion: store.MinVersion(),
		MaxVersion: store.MaxVersion(),
		Name:       s.raftServer.Name(),
		PeerURL:    s.Config.URL,
		ClientURL:  s.server.URL(),
	})
	log.Debugf("%s start as a leader", s.Config.Name)
	s.joinIndex = 1

	conf := NewClusterConfig()
	s.doCommand(&SetClusterConfigCommand{Config: conf})
	log.Debugf("%s sets cluster config as %v", s.Config.Name, conf)
}

func (s *PeerServer) doCommand(cmd raft.Command) {
	for {
		if _, err := s.raftServer.Do(cmd); err == nil {
			break
		}
	}
}

func (s *PeerServer) startAsFollower(cluster []string, retryTimes int) (error, error) {
	// start as a follower in a existing cluster
	for i := 0; ; i++ {
		joinErr, err := s.joinCluster(cluster)
		if err != nil {
			if i == retryTimes-1 {
				break
			}
			log.Infof("%v is unable to join the cluster using any of the peers %v at %dth time. Retrying in %.1f seconds", s.Config.Name, cluster, i, s.Config.RetryInterval)
			time.Sleep(time.Second * time.Duration(s.Config.RetryInterval))
			continue
		}
		if joinErr != nil {
			return joinErr, nil
		}
		return nil, nil
	}
	return nil, fmt.Errorf("Cannot join the cluster via given peers after %x retries", retryTimes)
}

// Upgradable checks whether all peers in a cluster support an upgrade to the next store version.
func (s *PeerServer) Upgradable() error {
	nextVersion := s.store.Version() + 1
	for _, peerURL := range s.registry.PeerURLs(s.raftServer.Leader(), s.Config.Name) {
		u, err := url.Parse(peerURL)
		if err != nil {
			return fmt.Errorf("PeerServer: Cannot parse URL: '%s' (%s)", peerURL, err)
		}

		url := (&url.URL{Host: u.Host, Scheme: s.Config.Scheme}).String()
		ok, err := s.client.CheckVersion(url, nextVersion)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("PeerServer: Version %d is not compatible with peer: %s", nextVersion, u.Host)
		}
	}

	return nil
}

// checkPeerAddressNonconflict checks whether the peer address has existed with different name.
func (s *PeerServer) checkPeerAddressNonconflict() bool {
	// there exists the (name, peer address) pair
	if peerURL, ok := s.registry.PeerURL(s.Config.Name); ok {
		if peerURL == s.Config.URL {
			return true
		}
	}

	// check all existing peer addresses
	peerURLs := s.registry.PeerURLs(s.raftServer.Leader(), s.Config.Name)
	for _, peerURL := range peerURLs {
		if peerURL == s.Config.URL {
			return false
		}
	}
	return true
}

// Helper function to do discovery and return results in expected format
func (s *PeerServer) handleDiscovery(discoverURL string) (peers []string, err error) {
	peers, err = discovery.Do(discoverURL, s.Config.Name, s.Config.URL, s.closeChan, &s.routineGroup)

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

// getKnownPeers gets the previous peers from log
func (s *PeerServer) getKnownPeers() []string {
	peers := s.registry.PeerURLs(s.raftServer.Leader(), s.Config.Name)
	log.Infof("Peer URLs in log: %s / %s (%s)", s.raftServer.Leader(), s.Config.Name, strings.Join(peers, ","))

	for i := range peers {
		u, err := url.Parse(peers[i])
		if err != nil {
			log.Debugf("getKnownPeers cannot parse url %v", peers[i])
		}
		peers[i] = u.Host
	}
	return peers
}

// removeSelfFromList removes url of the peerServer from the peer list
func (s *PeerServer) removeSelfFromList(peers []string) []string {
	// Remove its own peer address from the peer list to join
	u, err := url.Parse(s.Config.URL)
	if err != nil {
		log.Warnf("failed parsing self peer address %v", s.Config.URL)
		u = nil
	}
	newPeers := make([]string, 0)
	for _, v := range peers {
		if u == nil || v != u.Host {
			newPeers = append(newPeers, v)
		}
	}
	return newPeers
}

func (s *PeerServer) joinCluster(cluster []string) (error, error) {
	for _, peer := range cluster {
		if len(peer) == 0 {
			continue
		}

		joinErr, err := s.joinByPeer(s.raftServer, peer, s.Config.Scheme)
		if err != nil {
			log.Infof("%s attempted to join via %s failed: %s", s.Config.Name, peer, err)
			continue
		}

		if joinErr != nil {
			log.Infof("%s is rejected by the cluster via peer %s: %s", s.Config.Name, peer, joinErr)
			return joinErr, nil
		}
		log.Infof("%s joined the cluster via peer %s", s.Config.Name, peer)
		return nil, nil
	}

	return nil, fmt.Errorf("unreachable cluster")
}

// Send join requests to peer.
// The first return is why it is rejected by the cluster, the second one
// specifies the error in communication.
func (s *PeerServer) joinByPeer(server raft.Server, peer string, scheme string) (error, error) {
	u := (&url.URL{Host: peer, Scheme: scheme}).String()

	// Our version must match the leaders version
	version, err := s.client.GetVersion(u)
	if err != nil {
		log.Debugf("fail checking join version")
		return nil, err
	}
	if version < store.MinVersion() || version > store.MaxVersion() {
		log.Infof("fail passing version compatibility(%d-%d) using %d", store.MinVersion(), store.MaxVersion(), version)
		return fmt.Errorf("incompatible version"), nil
	}

	// Fetch current peer list
	machines, err := s.client.GetMachines(u)
	if err != nil {
		log.Debugf("fail getting machine messages")
		return nil, err
	}
	exist := false
	for _, machine := range machines {
		if machine.Name == server.Name() {
			exist = true
			// TODO(yichengq): cannot set join index for it.
			// Need discussion about the best way to do it.
			//
			// if machine.PeerURL == s.Config.URL {
			// 	log.Infof("has joined the cluster(%v) before", machines)
			// 	return nil
			// }
			break
		}
	}

	// Fetch cluster config to see whether exists some place.
	clusterConfig, err := s.client.GetClusterConfig(u)
	if err != nil {
		log.Debugf("fail getting cluster config")
		return nil, err
	}
	if !exist && clusterConfig.ActiveSize <= len(machines) {
		log.Infof("stop joining because the cluster is full with %d nodes", len(machines))
		return fmt.Errorf("out of quota"), nil
	}

	joinResp, err := s.client.AddMachine(u,
		&JoinCommandV2{
			MinVersion: store.MinVersion(),
			MaxVersion: store.MaxVersion(),
			Name:       server.Name(),
			PeerURL:    s.Config.URL,
			ClientURL:  s.server.URL(),
		})
	if err != nil {
		log.Debugf("fail on join request")
		return nil, err
	}
	s.joinIndex = joinResp.CommitIndex

	return nil, nil
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

func (s *PeerServer) daemon(f func()) {
	s.routineGroup.Add(1)
	go func() {
		defer s.routineGroup.Done()
		f()
	}()
}

func (s *PeerServer) monitorSnapshot() {
	for {
		timer := time.NewTimer(s.snapConf.checkingInterval)
		defer timer.Stop()
		select {
		case <-s.closeChan:
			return
		case <-timer.C:
		}

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
	ticker := time.NewTicker(time.Millisecond * 500)
	defer ticker.Stop()
	for {
		select {
		case <-s.closeChan:
			return
		case now := <-ticker.C:
			if s.raftServer.State() == raft.Leader {
				s.raftServer.Do(s.store.CommandFactory().CreateSyncCommand(now))
			}
		}
	}
}

// monitorTimeoutThreshold groups timeout threshold events together and prints
// them as a single log line.
func (s *PeerServer) monitorTimeoutThreshold() {
	for {
		select {
		case <-s.closeChan:
			return
		case value := <-s.timeoutThresholdChan:
			log.Infof("%s: warning: heartbeat near election timeout: %v", s.Config.Name, value)
		}

		timer := time.NewTimer(ThresholdMonitorTimeout)
		defer timer.Stop()
		select {
		case <-s.closeChan:
			return
		case <-timer.C:
		}
	}
}

// monitorActiveSize has the leader periodically check the status of cluster
// nodes and swaps them out for standbys as needed.
func (s *PeerServer) monitorActiveSize() {
	for {
		timer := time.NewTimer(ActiveMonitorTimeout)
		defer timer.Stop()
		select {
		case <-s.closeChan:
			return
		case <-timer.C:
		}

		// Ignore while this peer is not a leader.
		if s.raftServer.State() != raft.Leader {
			continue
		}

		// Retrieve target active size and actual active size.
		activeSize := s.ClusterConfig().ActiveSize
		peers := s.registry.Names()
		peerCount := len(peers)
		for i, peer := range peers {
			if peer == s.Config.Name {
				peers = append(peers[:i], peers[i+1:]...)
			}
		}

		// If we have more active nodes than we should then remove.
		if peerCount > activeSize {
			peer := peers[rand.Intn(len(peers))]
			log.Infof("%s: removing: %v", s.Config.Name, peer)
			if _, err := s.raftServer.Do(&RemoveCommandV2{Name: peer}); err != nil {
				log.Infof("%s: warning: remove error: %v", s.Config.Name, err)
			}
			continue
		}
	}
}

// monitorPeerActivity has the leader periodically for dead nodes and demotes them.
func (s *PeerServer) monitorPeerActivity() {
	for {
		timer := time.NewTimer(PeerActivityMonitorTimeout)
		defer timer.Stop()
		select {
		case <-s.closeChan:
			return
		case <-timer.C:
		}

		// Ignore while this peer is not a leader.
		if s.raftServer.State() != raft.Leader {
			continue
		}

		// Check last activity for all peers.
		now := time.Now()
		removeDelay := time.Duration(s.ClusterConfig().RemoveDelay) * time.Second
		peers := s.raftServer.Peers()
		for _, peer := range peers {
			// If the last response from the peer is longer than the promote delay
			// then automatically demote the peer.
			if !peer.LastActivity().IsZero() && now.Sub(peer.LastActivity()) > removeDelay {
				log.Infof("%s: removing node: %v; last activity %v ago", s.Config.Name, peer.Name, now.Sub(peer.LastActivity()))
				if _, err := s.raftServer.Do(&RemoveCommandV2{Name: peer.Name}); err != nil {
					log.Infof("%s: warning: autodemotion error: %v", s.Config.Name, err)
				}
				continue
			}
		}
	}
}
