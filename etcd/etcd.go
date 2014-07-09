package etcd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/coreos/etcd/config"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/store"
)

const (
	defaultHeartbeat = 1
	defaultElection  = 5

	defaultTickDuration = time.Millisecond * 100

	v2machineKVPrefix  = "/_etcd/machines"
	v2Prefix           = "/v2/keys"
	v2machinePrefix    = "/v2/machines"
	v2peersPrefix      = "/v2/peers"
	v2LeaderPrefix     = "/v2/leader"
	v2StoreStatsPrefix = "/v2/stats/store"

	v2configKVPrefix      = "/_etcd/config"
	v2adminConfigPrefix   = "/v2/admin/config"
	v2adminMachinesPrefix = "/v2/admin/machines/"

	raftPrefix = "/raft"
)

const (
	participant = iota
	standby
	stop
)

type Server struct {
	config *config.Config

	mode int

	id           int64
	pubAddr      string
	raftPubAddr  string
	nodes        map[string]bool
	tickDuration time.Duration

	proposal chan v2Proposal
	node     *v2Raft
	t        *transporter

	store.Store

	stop chan struct{}

	http.Handler
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
		nodes:        make(map[string]bool),
		tickDuration: defaultTickDuration,
		proposal:     make(chan v2Proposal),
		node: &v2Raft{
			Node:   raft.New(id, defaultHeartbeat, defaultElection),
			result: make(map[wait]chan interface{}),
		},
		t: newTransporter(tc),

		Store: store.New(),

		stop: make(chan struct{}),
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
	s.Handler = m
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
	close(s.stop)
	s.t.stop()
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
	d, err := json.Marshal(&raft.Config{s.id, s.raftPubAddr, []byte(s.pubAddr)})
	if err != nil {
		panic(err)
	}

	b, err := json.Marshal(&raft.Message{From: s.id, Type: 2, Entries: []raft.Entry{{Type: 1, Data: d}}})
	if err != nil {
		panic(err)
	}

	for seed := range s.nodes {
		if err := s.t.send(seed+raftPrefix, b); err != nil {
			log.Println(err)
			continue
		}
		// todo(xiangli) WAIT for join to be committed or retry...
		break
	}
	s.run()
}

func (s *Server) run() {
	for {
		switch s.mode {
		case participant:
			s.runParticipant()
		case standby:
			s.runStandby()
		case stop:
			return
		default:
			panic("unsupport mode")
		}
	}
}

func (s *Server) runParticipant() {
	node := s.node
	recv := s.t.recv
	ticker := time.NewTicker(s.tickDuration)
	v2SyncTicker := time.NewTicker(time.Millisecond * 500)

	var proposal chan v2Proposal
	for {
		if node.HasLeader() {
			proposal = s.proposal
		} else {
			proposal = nil
		}
		select {
		case p := <-proposal:
			node.Propose(p)
		case msg := <-recv:
			node.Step(*msg)
		case <-ticker.C:
			node.Tick()
		case <-v2SyncTicker.C:
			node.Sync()
		case <-s.stop:
			log.Printf("Node: %d stopped\n", s.id)
			s.mode = stop
			return
		}
		s.apply(node.Next())
		s.send(node.Msgs())
	}
}

func (s *Server) runStandby() {
	panic("unimplemented")
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
			s.nodes[cfg.Addr] = true
			p := path.Join(v2machineKVPrefix, fmt.Sprint(cfg.NodeId))
			s.Store.Set(p, false, fmt.Sprintf("raft=%v&etcd=%v", cfg.Addr, string(cfg.Context)), store.Permanent)
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

func (s *Server) fetchAddr(nodeId int64) error {
	for seed := range s.nodes {
		if err := s.t.fetchAddr(seed, nodeId); err == nil {
			return nil
		}
	}
	return fmt.Errorf("cannot fetch the address of node %d", nodeId)
}
