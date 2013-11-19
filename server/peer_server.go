package server

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
	"github.com/gorilla/mux"
)

type PeerServer struct {
	raftServer     raft.Server
	server         *Server
	httpServer     *http.Server
	listener       net.Listener
	joinIndex      uint64
	name           string
	url            string
	bindAddr       string
	tlsConf        *TLSConfig
	tlsInfo        *TLSInfo
	followersStats *raftFollowersStats
	serverStats    *raftServerStats
	registry       *Registry
	store          store.Store
	snapConf       *snapshotConf
	MaxClusterSize int
	RetryTimes     int
}

// TODO: find a good policy to do snapshot
type snapshotConf struct {
	// Etcd will check if snapshot is need every checkingInterval
	checkingInterval time.Duration

	// The number of writes when the last snapshot happened
	lastWrites uint64

	// If the incremental number of writes since the last snapshot
	// exceeds the write Threshold, etcd will do a snapshot
	writesThr uint64
}

func NewPeerServer(name string, path string, url string, bindAddr string, tlsConf *TLSConfig, tlsInfo *TLSInfo, registry *Registry, store store.Store, snapshotCount int) *PeerServer {
	s := &PeerServer{
		name:     name,
		url:      url,
		bindAddr: bindAddr,
		tlsConf:  tlsConf,
		tlsInfo:  tlsInfo,
		registry: registry,
		store:    store,
		snapConf: &snapshotConf{time.Second * 3, 0, uint64(snapshotCount)},
		followersStats: &raftFollowersStats{
			Leader:    name,
			Followers: make(map[string]*raftFollowerStats),
		},
		serverStats: &raftServerStats{
			StartTime: time.Now(),
			sendRateQueue: &statsQueue{
				back: -1,
			},
			recvRateQueue: &statsQueue{
				back: -1,
			},
		},
	}

	// Create transporter for raft
	raftTransporter := newTransporter(tlsConf.Scheme, tlsConf.Client, s)

	// Create raft server
	raftServer, err := raft.NewServer(name, path, raftTransporter, s.store, s, "")
	if err != nil {
		log.Fatal(err)
	}

	s.raftServer = raftServer

	return s
}

// Start the raft server
func (s *PeerServer) ListenAndServe(snapshot bool, cluster []string) error {
	// LoadSnapshot
	if snapshot {
		err := s.raftServer.LoadSnapshot()

		if err == nil {
			log.Debugf("%s finished load snapshot", s.name)
		} else {
			log.Debug(err)
		}
	}

	s.raftServer.SetElectionTimeout(ElectionTimeout)
	s.raftServer.SetHeartbeatTimeout(HeartbeatTimeout)

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
		cluster = s.registry.PeerURLs(s.raftServer.Leader(), s.name)
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

		log.Debugf("%s restart as a follower", s.name)
	}

	go s.monitorSync()

	// open the snapshot
	if snapshot {
		go s.monitorSnapshot()
	}

	// start to response to raft requests
	return s.startTransport(s.tlsConf.Scheme, s.tlsConf.Server)
}

// Overridden version of net/http added so we can manage the listener.
func (s *PeerServer) listenAndServe() error {
	addr := s.httpServer.Addr
	if addr == "" {
		addr = ":http"
	}
	l, e := net.Listen("tcp", addr)
	if e != nil {
		return e
	}
	s.listener = l
	return s.httpServer.Serve(l)
}

// Overridden version of net/http added so we can manage the listener.
func (s *PeerServer) listenAndServeTLS(certFile, keyFile string) error {
	addr := s.httpServer.Addr
	if addr == "" {
		addr = ":https"
	}
	config := &tls.Config{}
	if s.httpServer.TLSConfig != nil {
		*config = *s.httpServer.TLSConfig
	}
	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}

	var err error
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	conn, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	tlsListener := tls.NewListener(conn, config)
	s.listener = tlsListener
	return s.httpServer.Serve(tlsListener)
}

// Stops the server.
func (s *PeerServer) Close() {
	if s.listener != nil {
		s.listener.Close()
		s.listener = nil
	}
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
		_, err := s.raftServer.Do(NewJoinCommand(store.MinVersion(), store.MaxVersion(), s.raftServer.Name(), s.url, s.server.URL()))
		if err == nil {
			break
		}
	}
	log.Debugf("%s start as a leader", s.name)
}

func (s *PeerServer) startAsFollower(cluster []string) {
	// start as a follower in a existing cluster
	for i := 0; i < s.RetryTimes; i++ {
		ok := s.joinCluster(cluster)
		if ok {
			return
		}
		log.Warnf("cannot join to cluster via given peers, retry in %d seconds", RetryInterval)
		time.Sleep(time.Second * RetryInterval)
	}

	log.Fatalf("Cannot join the cluster via given peers after %x retries", s.RetryTimes)
}

// Start to listen and response raft command
func (s *PeerServer) startTransport(scheme string, tlsConf tls.Config) error {
	log.Infof("raft server [name %s, listen on %s, advertised url %s]", s.name, s.bindAddr, s.url)

	router := mux.NewRouter()

	s.httpServer = &http.Server{
		Handler:   router,
		TLSConfig: &tlsConf,
		Addr:      s.bindAddr,
	}

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

	if scheme == "http" {
		return s.listenAndServe()
	} else {
		return s.listenAndServeTLS(s.tlsInfo.CertFile, s.tlsInfo.KeyFile)
	}

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
	for _, peerURL := range s.registry.PeerURLs(s.raftServer.Leader(), s.name) {
		u, err := url.Parse(peerURL)
		if err != nil {
			return fmt.Errorf("PeerServer: Cannot parse URL: '%s' (%s)", peerURL, err)
		}

		t, _ := s.raftServer.Transporter().(*transporter)
		checkURL := (&url.URL{Host: u.Host, Scheme: s.tlsConf.Scheme, Path: fmt.Sprintf("/version/%d/check", nextVersion)}).String()
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

		err := s.joinByPeer(s.raftServer, peer, s.tlsConf.Scheme)
		if err == nil {
			log.Debugf("%s success join to the cluster via peer %s", s.name, peer)
			return true

		} else {
			if _, ok := err.(etcdErr.Error); ok {
				log.Fatal(err)
			}

			log.Debugf("cannot join to cluster via peer %s %s", peer, err)
		}
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

	json.NewEncoder(&b).Encode(NewJoinCommand(store.MinVersion(), store.MaxVersion(), server.Name(), s.url, s.server.URL()))

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
				json.NewEncoder(&b).Encode(NewJoinCommand(store.MinVersion(), store.MaxVersion(), server.Name(), s.url, s.server.URL()))
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

func (s *PeerServer) monitorSnapshot() {
	for {
		time.Sleep(s.snapConf.checkingInterval)
		currentWrites := s.store.TotalTransactions() - s.snapConf.lastWrites
		if uint64(currentWrites) > s.snapConf.writesThr {
			s.raftServer.TakeSnapshot()
			s.snapConf.lastWrites = s.store.TotalTransactions()
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
