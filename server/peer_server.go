package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/raft"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/metrics"
	"github.com/coreos/etcd/store"
)

const ThresholdMonitorTimeout = 5 * time.Second

type PeerServerConfig struct {
	Name           string
	Scheme         string
	URL            string
	SnapshotCount  int
	MaxClusterSize int
	RetryTimes     int
	RetryInterval  float64
}

type PeerServer struct {
	Config		PeerServerConfig
	raftServer	raft.Server
	server		*Server
	joinIndex	uint64
	followersStats	*raftFollowersStats
	serverStats	*raftServerStats
	registry	*Registry
	store		store.Store
	snapConf	*snapshotConf

	closeChan		chan bool
	timeoutThresholdChan	chan interface{}

	metrics	*metrics.Bucket
}

// TODO: find a good policy to do snapshot
type snapshotConf struct {
	// Etcd will check if snapshot is need every checkingInterval
	checkingInterval	time.Duration

	// The index when the last snapshot happened
	lastIndex	uint64

	// If the incremental number of index since the last snapshot
	// exceeds the snapshot Threshold, etcd will do a snapshot
	snapshotThr	uint64
}

func NewPeerServer(psConfig PeerServerConfig, registry *Registry, store store.Store, mb *metrics.Bucket, followersStats *raftFollowersStats, serverStats *raftServerStats) *PeerServer {
	s := &PeerServer{
		Config:		psConfig,
		registry:	registry,
		store:		store,
		followersStats:	followersStats,
		serverStats:	serverStats,

		timeoutThresholdChan:	make(chan interface{}, 1),

		metrics:	mb,
	}

	return s
}

func (s *PeerServer) SetRaftServer(raftServer raft.Server) {
	s.snapConf = &snapshotConf{
		checkingInterval:	time.Second * 3,
		// this is not accurate, we will update raft to provide an api
		lastIndex:	raftServer.CommitIndex(),
		snapshotThr:	uint64(s.Config.SnapshotCount),
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

// Start the raft server
func (s *PeerServer) Start(snapshot bool, cluster []string) error {
	// LoadSnapshot
	if snapshot {
		err := s.raftServer.LoadSnapshot()

		if err == nil {
			log.Debugf("%s finished load snapshot", s.Config.Name)
		} else {
			log.Debug(err)
		}
	}

	s.raftServer.Start()

	if s.raftServer.IsLogEmpty() {
		// start as a leader in a new cluster
		if len(cluster) == 0 {
			s.startAsLeader()
		} else {
			s.startAsFollower(cluster)
		}

	} else {
		// Rejoin the previous cluster
		cluster = s.registry.PeerURLs(s.raftServer.Leader(), s.Config.Name)
		for i := 0; i < len(cluster); i++ {
			u, err := url.Parse(cluster[i])
			if err != nil {
				log.Debug("rejoin cannot parse url: ", err)
			}
			cluster[i] = u.Host
		}
		ok := s.joinCluster(cluster)
		if !ok {
			log.Warn("the entire cluster is down! this peer will restart the cluster.")
		}

		log.Debugf("%s restart as a follower", s.Config.Name)
	}

	s.closeChan = make(chan bool)

	go s.monitorSync()
	go s.monitorTimeoutThreshold(s.closeChan)

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
	router.HandleFunc("/remove/{name:.+}", s.RemoveHttpHandler)
	router.HandleFunc("/vote", s.VoteHttpHandler)
	router.HandleFunc("/log", s.GetLogHttpHandler)
	router.HandleFunc("/log/append", s.AppendEntriesHttpHandler)
	router.HandleFunc("/snapshot", s.SnapshotHttpHandler)
	router.HandleFunc("/snapshotRecovery", s.SnapshotRecoveryHttpHandler)
	router.HandleFunc("/etcdURL", s.EtcdURLHttpHandler)

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
	// leader need to join self as a peer
	for {
		_, err := s.raftServer.Do(NewJoinCommand(store.MinVersion(), store.MaxVersion(), s.raftServer.Name(), s.Config.URL, s.server.URL()))
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
			return
		}
		log.Warnf("Unable to join the cluster using any of the peers %v. Retrying in %.1f seconds", cluster, s.Config.RetryInterval)
		time.Sleep(time.Second * time.Duration(s.Config.RetryInterval))
	}

	log.Fatalf("Cannot join the cluster via given peers after %x retries", s.Config.RetryTimes)
}

// getVersion fetches the peer version of a cluster.
func getVersion(t *transporter, versionURL url.URL) (int, error) {
	resp, req, err := t.Get(versionURL.String())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	t.CancelWhenTimeout(req)
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
	var b bytes.Buffer

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

	json.NewEncoder(&b).Encode(NewJoinCommand(store.MinVersion(), store.MaxVersion(), server.Name(), s.Config.URL, s.server.URL()))

	joinURL := url.URL{Host: peer, Scheme: scheme, Path: "/join"}

	log.Debugf("Send Join Request to %s", joinURL.String())

	resp, req, err := t.Post(joinURL.String(), &b)

	for {
		if err != nil {
			return fmt.Errorf("Unable to join: %v", err)
		}
		if resp != nil {
			defer resp.Body.Close()

			t.CancelWhenTimeout(req)

			if resp.StatusCode == http.StatusOK {
				b, _ := ioutil.ReadAll(resp.Body)
				s.joinIndex, _ = binary.Uvarint(b)
				return nil
			}
			if resp.StatusCode == http.StatusTemporaryRedirect {
				address := resp.Header.Get("Location")
				log.Debugf("Send Join Request to %s", address)
				json.NewEncoder(&b).Encode(NewJoinCommand(store.MinVersion(), store.MaxVersion(), server.Name(), s.Config.URL, s.server.URL()))
				resp, req, err = t.Post(address, &b)

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
