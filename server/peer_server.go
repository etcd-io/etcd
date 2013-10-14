package server

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
)

type PeerServer struct {
	*raft.Server
	server         *Server
	joinIndex      uint64
	name           string
	url            string
	listenHost     string
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

func NewPeerServer(name string, path string, url string, listenHost string, tlsConf *TLSConfig, tlsInfo *TLSInfo, registry *Registry, store store.Store) *PeerServer {
	s := &PeerServer{
		name:       name,
		url:        url,
		listenHost: listenHost,
		tlsConf:    tlsConf,
		tlsInfo:    tlsInfo,
		registry:   registry,
		store:      store,
		snapConf:   &snapshotConf{time.Second * 3, 0, 20 * 1000},
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
	server, err := raft.NewServer(name, path, raftTransporter, s.store, s, "")
	if err != nil {
		log.Fatal(err)
	}

	s.Server = server

	return s
}

// Start the raft server
func (s *PeerServer) ListenAndServe(snapshot bool, cluster []string) {
	// LoadSnapshot
	if snapshot {
		err := s.LoadSnapshot()

		if err == nil {
			log.Debugf("%s finished load snapshot", s.name)
		} else {
			log.Debug(err)
		}
	}

	s.SetElectionTimeout(ElectionTimeout)
	s.SetHeartbeatTimeout(HeartbeatTimeout)

	s.Start()

	if s.IsLogEmpty() {
		// start as a leader in a new cluster
		if len(cluster) == 0 {
			s.startAsLeader()
		} else {
			s.startAsFollower(cluster)
		}

	} else {
		// Rejoin the previous cluster
		cluster = s.registry.PeerURLs(s.Leader(), s.name)
		for i := 0; i < len(cluster); i++ {
			u, err := url.Parse(cluster[i])
			if err != nil {
				log.Debug("rejoin cannot parse url: ", err)
			}
			cluster[i] = u.Host
		}
		ok := s.joinCluster(cluster)
		if !ok {
			log.Warn("the entire cluster is down! this machine will restart the cluster.")
		}

		log.Debugf("%s restart as a follower", s.name)
	}

	// open the snapshot
	if snapshot {
		go s.monitorSnapshot()
	}

	// start to response to raft requests
	go s.startTransport(s.tlsConf.Scheme, s.tlsConf.Server)

}

// Retrieves the underlying Raft server.
func (s *PeerServer) RaftServer() *raft.Server {
	return s.Server
}

// Associates the client server with the peer server.
func (s *PeerServer) SetServer(server *Server) {
	s.server = server
}

func (s *PeerServer) startAsLeader() {
	// leader need to join self as a peer
	for {
		_, err := s.Do(NewJoinCommand(PeerVersion, s.Name(), s.url, s.server.URL()))
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
		log.Warnf("cannot join to cluster via given machines, retry in %d seconds", RetryInterval)
		time.Sleep(time.Second * RetryInterval)
	}

	log.Fatalf("Cannot join the cluster via given machines after %x retries", s.RetryTimes)
}

// Start to listen and response raft command
func (s *PeerServer) startTransport(scheme string, tlsConf tls.Config) {
	log.Infof("raft server [name %s, listen on %s, advertised url %s]", s.name, s.listenHost, s.url)

	raftMux := http.NewServeMux()

	server := &http.Server{
		Handler:   raftMux,
		TLSConfig: &tlsConf,
		Addr:      s.listenHost,
	}

	// internal commands
	raftMux.HandleFunc("/name", s.NameHttpHandler)
	raftMux.HandleFunc("/version", s.RaftVersionHttpHandler)
	raftMux.HandleFunc("/join", s.JoinHttpHandler)
	raftMux.HandleFunc("/remove/", s.RemoveHttpHandler)
	raftMux.HandleFunc("/vote", s.VoteHttpHandler)
	raftMux.HandleFunc("/log", s.GetLogHttpHandler)
	raftMux.HandleFunc("/log/append", s.AppendEntriesHttpHandler)
	raftMux.HandleFunc("/snapshot", s.SnapshotHttpHandler)
	raftMux.HandleFunc("/snapshotRecovery", s.SnapshotRecoveryHttpHandler)
	raftMux.HandleFunc("/etcdURL", s.EtcdURLHttpHandler)

	if scheme == "http" {
		log.Fatal(server.ListenAndServe())
	} else {
		log.Fatal(server.ListenAndServeTLS(s.tlsInfo.CertFile, s.tlsInfo.KeyFile))
	}

}

// getVersion fetches the raft version of a peer. This works for now but we
// will need to do something more sophisticated later when we allow mixed
// version clusters.
func getVersion(t *transporter, versionURL url.URL) (string, error) {
	resp, req, err := t.Get(versionURL.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	t.CancelWhenTimeout(req)

	body, err := ioutil.ReadAll(resp.Body)

	return string(body), nil
}

func (s *PeerServer) joinCluster(cluster []string) bool {
	for _, machine := range cluster {
		if len(machine) == 0 {
			continue
		}

		err := s.joinByMachine(s.Server, machine, s.tlsConf.Scheme)
		if err == nil {
			log.Debugf("%s success join to the cluster via machine %s", s.name, machine)
			return true

		} else {
			if _, ok := err.(etcdErr.Error); ok {
				log.Fatal(err)
			}

			log.Debugf("cannot join to cluster via machine %s %s", machine, err)
		}
	}
	return false
}

// Send join requests to machine.
func (s *PeerServer) joinByMachine(server *raft.Server, machine string, scheme string) error {
	var b bytes.Buffer

	// t must be ok
	t, _ := server.Transporter().(*transporter)

	// Our version must match the leaders version
	versionURL := url.URL{Host: machine, Scheme: scheme, Path: "/version"}
	version, err := getVersion(t, versionURL)
	if err != nil {
		return fmt.Errorf("Error during join version check: %v", err)
	}

	// TODO: versioning of the internal protocol. See:
	// Documentation/internatl-protocol-versioning.md
	if version != PeerVersion {
		return fmt.Errorf("Unable to join: internal version mismatch, entire cluster must be running identical versions of etcd")
	}

	json.NewEncoder(&b).Encode(NewJoinCommand(PeerVersion, server.Name(), s.url, s.server.URL()))

	joinURL := url.URL{Host: machine, Scheme: scheme, Path: "/join"}

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
				json.NewEncoder(&b).Encode(NewJoinCommand(PeerVersion, server.Name(), s.url, s.server.URL()))
				resp, req, err = t.Post(address, &b)

			} else if resp.StatusCode == http.StatusBadRequest {
				log.Debug("Reach max number machines in the cluster")
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
	if s.State() == raft.Leader {
		b, _ := json.Marshal(s.followersStats)
		return b
	}
	return nil
}

func (s *PeerServer) monitorSnapshot() {
	for {
		time.Sleep(s.snapConf.checkingInterval)
		currentWrites := 0
		if uint64(currentWrites) > s.snapConf.writesThr {
			s.TakeSnapshot()
			s.snapConf.lastWrites = 0
		}
	}
}

func (s *PeerServer) dispatch(c raft.Command, w http.ResponseWriter, req *http.Request) error {
	if s.State() == raft.Leader {
		result, err := s.Do(c)
		if err != nil {
			return err
		}

		if result == nil {
			return etcdErr.NewError(300, "Empty result from raft", store.UndefIndex, store.UndefTerm)
		}

		// response for raft related commands[join/remove]
		if b, ok := result.([]byte); ok {
			w.WriteHeader(http.StatusOK)
			w.Write(b)
			return nil
		}

		var b []byte
		if strings.HasPrefix(req.URL.Path, "/v1") {
			b, _ = json.Marshal(result.(*store.Event).Response())
		} else {
			b, _ = json.Marshal(result.(*store.Event))
		}
		w.WriteHeader(http.StatusOK)
		w.Write(b)

		return nil

	} else {
		leader := s.Leader()

		// No leader available.
		if leader == "" {
			return etcdErr.NewError(300, "", store.UndefIndex, store.UndefTerm)
		}

		var url string
		switch c.(type) {
		case *JoinCommand, *RemoveCommand:
			url, _ = s.registry.PeerURL(leader)
		default:
			url, _ = s.registry.ClientURL(leader)
		}
		redirect(url, w, req)

		return nil
	}
}
