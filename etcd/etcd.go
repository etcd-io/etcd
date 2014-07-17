package etcd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/coreos/etcd/config"
	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/store"
)

const (
	defaultHeartbeat = 1
	defaultElection  = 5

	maxBufferedProposal = 128

	defaultTickDuration = time.Millisecond * 100

	v2machineKVPrefix = "/_etcd/machines"
	v2configKVPrefix  = "/_etcd/config"

	v2Prefix              = "/v2/keys"
	v2machinePrefix       = "/v2/machines"
	v2peersPrefix         = "/v2/peers"
	v2LeaderPrefix        = "/v2/leader"
	v2StoreStatsPrefix    = "/v2/stats/store"
	v2adminConfigPrefix   = "/v2/admin/config"
	v2adminMachinesPrefix = "/v2/admin/machines/"

	raftPrefix = "/raft"
)

const (
	participant = iota
	standby
	stop
)

var (
	tmpErr            = fmt.Errorf("try again")
	raftStopErr       = fmt.Errorf("raft is stopped")
	noneId      int64 = -1
)

type Server struct {
	config *config.Config

	mode int

	id           int64
	pubAddr      string
	raftPubAddr  string
	tickDuration time.Duration

	nodes  map[string]bool
	client *v2client
	t      *transporter
	node   *v2Raft
	store.Store

	// participant mode vars
	proposal    chan v2Proposal
	addNodeC    chan raft.Config
	removeNodeC chan raft.Config

	// standby mode vars
	leader      int64
	leaderAddr  string
	clusterConf *config.ClusterConfig

	modeC chan int
	stop  chan struct{}

	participantHandler http.Handler
	standbyHandler     http.Handler
}

func New(c *config.Config, id int64) *Server {
	if err := c.Sanitize(); err != nil {
		log.Fatalf("failed sanitizing configuration: %v", err)
	}

	tc := &tls.Config{
		InsecureSkipVerify: true,
	}
	var err error
	if c.PeerTLSInfo().Scheme() == "https" {
		tc, err = c.PeerTLSInfo().ClientConfig()
		if err != nil {
			log.Fatal("failed to create raft transporter tls:", err)
		}
	}

	s := &Server{
		config:       c,
		id:           id,
		pubAddr:      c.Addr,
		raftPubAddr:  c.Peer.Addr,
		tickDuration: defaultTickDuration,

		nodes:  make(map[string]bool),
		client: newClient(tc),
		t:      newTransporter(tc),
		node: &v2Raft{
			Node:   raft.New(id, defaultHeartbeat, defaultElection),
			result: make(map[wait]chan interface{}),
		},
		Store: store.New(),

		modeC: make(chan int, 10),
		stop:  make(chan struct{}),
	}

	for _, seed := range c.Peers {
		s.nodes[seed] = true
	}

	m := http.NewServeMux()
	m.Handle(v2Prefix+"/", handlerErr(s.serveValue))
	m.Handle(v2machinePrefix, handlerErr(s.serveMachines))
	m.Handle(v2peersPrefix, handlerErr(s.serveMachines))
	m.Handle(v2LeaderPrefix, handlerErr(s.serveLeader))
	m.Handle(v2StoreStatsPrefix, handlerErr(s.serveStoreStats))
	m.Handle(v2adminConfigPrefix, handlerErr(s.serveAdminConfig))
	m.Handle(v2adminMachinesPrefix, handlerErr(s.serveAdminMachines))
	s.participantHandler = m
	m = http.NewServeMux()
	m.Handle("/", handlerErr(s.serveRedirect))
	s.standbyHandler = m
	return s
}

func (s *Server) SetTick(d time.Duration) {
	s.tickDuration = d
}

func (s *Server) RaftHandler() http.Handler {
	return s.t
}

func (s *Server) ClusterConfig() *config.ClusterConfig {
	c := config.NewClusterConfig()
	// This is used for backward compatibility because it doesn't
	// set cluster config in older version.
	if e, err := s.Get(v2configKVPrefix, false, false); err == nil {
		json.Unmarshal([]byte(*e.Node.Value), c)
	}
	return c
}

func (s *Server) Run() {
	if len(s.config.Peers) == 0 {
		s.Bootstrap()
	} else {
		s.Join()
	}
}

func (s *Server) Stop() {
	if s.mode == stop {
		return
	}
	s.mode = stop
	s.t.closeConnections()
	s.client.CloseConnections()
	close(s.stop)
}

func (s *Server) Bootstrap() {
	log.Println("starting a bootstrap node")
	s.node.Campaign()
	s.node.Add(s.id, s.raftPubAddr, []byte(s.pubAddr))
	s.apply(s.node.Next())
	s.run()
}

func (s *Server) Join() {
	log.Println("joining cluster via peers", s.config.Peers)
	info := &context{
		MinVersion: store.MinVersion(),
		MaxVersion: store.MaxVersion(),
		ClientURL:  s.pubAddr,
		PeerURL:    s.raftPubAddr,
	}

	url := ""
	for i := 0; i < 5; i++ {
		for seed := range s.nodes {
			if err := s.client.AddMachine(seed, fmt.Sprint(s.id), info); err == nil {
				url = seed
				break
			} else {
				log.Println(err)
			}
		}
		if url != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.nodes = map[string]bool{url: true}

	s.run()
}

func (s *Server) Add(id int64, raftPubAddr string, pubAddr string) error {
	p := path.Join(v2machineKVPrefix, fmt.Sprint(id))

	_, err := s.Get(p, false, false)
	if err == nil {
		return nil
	}
	if v, ok := err.(*etcdErr.Error); !ok || v.ErrorCode != etcdErr.EcodeKeyNotFound {
		return err
	}

	w, err := s.Watch(p, true, false, 0)
	if err != nil {
		log.Println("add error:", err)
		return tmpErr
	}

	if s.mode != participant {
		return raftStopErr
	}
	select {
	case s.addNodeC <- raft.Config{NodeId: id, Addr: raftPubAddr, Context: []byte(pubAddr)}:
	default:
		w.Remove()
		log.Println("unable to send out addNode proposal")
		return tmpErr
	}

	select {
	case v := <-w.EventChan:
		if v.Action == store.Set {
			return nil
		}
		log.Println("add error: action =", v.Action)
		return tmpErr
	case <-time.After(6 * defaultHeartbeat * s.tickDuration):
		w.Remove()
		log.Println("add error: wait timeout")
		return tmpErr
	}
}

func (s *Server) Remove(id int64) error {
	p := path.Join(v2machineKVPrefix, fmt.Sprint(id))

	v, err := s.Get(p, false, false)
	if err != nil {
		return nil
	}

	if s.mode != participant {
		return raftStopErr
	}
	select {
	case s.removeNodeC <- raft.Config{NodeId: id}:
	default:
		log.Println("unable to send out removeNode proposal")
		return tmpErr
	}

	// TODO(xiangli): do not need to watch if the
	// removal target is self
	w, err := s.Watch(p, true, false, v.Index()+1)
	if err != nil {
		log.Println("remove error:", err)
		return tmpErr
	}

	select {
	case v := <-w.EventChan:
		if v.Action == store.Delete {
			return nil
		}
		log.Println("remove error: action =", v.Action)
		return tmpErr
	case <-time.After(6 * defaultHeartbeat * s.tickDuration):
		w.Remove()
		log.Println("remove error: wait timeout")
		return tmpErr
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch s.mode {
	case participant:
		s.participantHandler.ServeHTTP(w, r)
	case standby:
		s.standbyHandler.ServeHTTP(w, r)
	case stop:
		http.Error(w, "server is stopped", http.StatusInternalServerError)
	}
}

func (s *Server) initParticipant() {
	s.proposal = make(chan v2Proposal, maxBufferedProposal)
	s.addNodeC = make(chan raft.Config, 1)
	s.removeNodeC = make(chan raft.Config, 1)
	s.t.start()
}

func (s *Server) initStandby() {
	s.leader = noneId
	s.leaderAddr = ""
	s.clusterConf = config.NewClusterConfig()
}

func (s *Server) run() {
	for {
		select {
		case s.modeC <- s.mode:
		default:
		}

		switch s.mode {
		case participant:
			s.initParticipant()
			s.runParticipant()
		case standby:
			s.initStandby()
			s.runStandby()
		case stop:
			return
		default:
			panic("unsupport mode")
		}
	}
}

func (s *Server) runParticipant() {
	defer func() {
		s.node.StopProposalWaiters()
		s.t.stop()
	}()
	node := s.node
	recv := s.t.recv
	ticker := time.NewTicker(s.tickDuration)
	v2SyncTicker := time.NewTicker(time.Millisecond * 500)

	var proposal chan v2Proposal
	var addNodeC, removeNodeC chan raft.Config
	for {
		if node.HasLeader() {
			proposal = s.proposal
			addNodeC = s.addNodeC
			removeNodeC = s.removeNodeC
		} else {
			proposal = nil
			addNodeC = nil
			removeNodeC = nil
		}
		select {
		case p := <-proposal:
			node.Propose(p)
		case c := <-addNodeC:
			node.UpdateConf(raft.AddNode, &c)
		case c := <-removeNodeC:
			node.UpdateConf(raft.RemoveNode, &c)
		case msg := <-recv:
			node.Step(*msg)
		case <-ticker.C:
			node.Tick()
		case <-v2SyncTicker.C:
			node.Sync()
		case <-s.stop:
			log.Printf("Node: %d stopped\n", s.id)
			return
		}
		s.apply(node.Next())
		s.send(node.Msgs())
		if node.IsRemoved() {
			break
		}
	}

	log.Printf("Node: %d removed to standby mode\n", s.id)
	s.mode = standby
	return
}

func (s *Server) runStandby() {
	syncDuration := time.Duration(int64(s.clusterConf.SyncInterval * float64(time.Second)))
	if err := s.syncCluster(); err != nil {
		log.Println("standby sync:", err)
	}
	for {
		select {
		case <-time.After(syncDuration):
		case <-s.stop:
			log.Printf("Node: %d stopped\n", s.id)
			return
		}

		if err := s.syncCluster(); err != nil {
			log.Println("standby sync:", err)
			continue
		}
		if s.clusterConf.ActiveSize <= len(s.nodes) {
			continue
		}
		if err := s.joinByPeer(s.leaderAddr); err != nil {
			log.Println("standby join:", err)
			continue
		}
		break
	}

	log.Printf("Node: %d removed to participant mode\n", s.id)
	// TODO(yichengq): use old v2Raft
	// 1. reject proposal in leader state when sm is removed
	// 2. record removeIndex in node to ignore msgDenial and old removal
	s.node = &v2Raft{
		Node:   raft.New(s.id, defaultHeartbeat, defaultElection),
		result: make(map[wait]chan interface{}),
	}
	s.Store = store.New()
	s.mode = participant
	return
}

func (s *Server) apply(ents []raft.Entry) {
	offset := s.node.Applied() - int64(len(ents)) + 1
	for i, ent := range ents {
		switch ent.Type {
		// expose raft entry type
		case raft.Normal:
			if len(ent.Data) == 0 {
				continue
			}
			s.v2apply(offset+int64(i), ent)
		case raft.AddNode:
			cfg := new(raft.Config)
			if err := json.Unmarshal(ent.Data, cfg); err != nil {
				log.Println(err)
				break
			}
			if err := s.t.set(cfg.NodeId, cfg.Addr); err != nil {
				log.Println(err)
				break
			}
			log.Printf("Add Node %x %v %v\n", cfg.NodeId, cfg.Addr, string(cfg.Context))
			p := path.Join(v2machineKVPrefix, fmt.Sprint(cfg.NodeId))
			if _, err := s.Store.Set(p, false, fmt.Sprintf("raft=%v&etcd=%v", cfg.Addr, string(cfg.Context)), store.Permanent); err == nil {
				s.nodes[cfg.Addr] = true
			}
		case raft.RemoveNode:
			cfg := new(raft.Config)
			if err := json.Unmarshal(ent.Data, cfg); err != nil {
				log.Println(err)
				break
			}
			log.Printf("Remove Node %x\n", cfg.NodeId)
			delete(s.nodes, s.fetchAddrFromStore(cfg.NodeId))
			p := path.Join(v2machineKVPrefix, fmt.Sprint(cfg.NodeId))
			s.Store.Delete(p, false, false)
		default:
			panic("unimplemented")
		}
	}
}

func (s *Server) send(msgs []raft.Message) {
	for i := range msgs {
		data, err := json.Marshal(msgs[i])
		if err != nil {
			// todo(xiangli): error handling
			log.Fatal(err)
		}
		// todo(xiangli): reuse routines and limit the number of sending routines
		// sync.Pool?
		go func(i int) {
			var err error
			if err = s.t.sendTo(msgs[i].To, data); err == nil {
				return
			}
			if err == errUnknownNode {
				err = s.fetchAddr(msgs[i].To)
			}
			if err == nil {
				err = s.t.sendTo(msgs[i].To, data)
			}
			if err != nil {
				log.Println(err)
			}
		}(i)
	}
}

func (s *Server) setClusterConfig(c *config.ClusterConfig) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if _, err := s.Set(v2configKVPrefix, false, string(b), store.Permanent); err != nil {
		return err
	}
	return nil
}

func (s *Server) fetchAddr(nodeId int64) error {
	for seed := range s.nodes {
		if err := s.t.fetchAddr(seed, nodeId); err == nil {
			return nil
		}
	}
	return fmt.Errorf("cannot fetch the address of node %d", nodeId)
}

func (s *Server) fetchAddrFromStore(nodeId int64) string {
	p := path.Join(v2machineKVPrefix, fmt.Sprint(nodeId))
	if ev, err := s.Get(p, false, false); err == nil {
		if m, err := url.ParseQuery(*ev.Node.Value); err == nil {
			return m["raft"][0]
		}
	}
	return ""
}

func (s *Server) joinByPeer(addr string) error {
	info := &context{
		MinVersion: store.MinVersion(),
		MaxVersion: store.MaxVersion(),
		ClientURL:  s.pubAddr,
		PeerURL:    s.raftPubAddr,
	}
	if err := s.client.AddMachine(s.leaderAddr, fmt.Sprint(s.id), info); err != nil {
		return err
	}
	return nil
}
