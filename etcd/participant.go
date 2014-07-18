package etcd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

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
)

var (
	tmpErr      = fmt.Errorf("try again")
	stopErr     = fmt.Errorf("server is stopped")
	raftStopErr = fmt.Errorf("raft is stopped")
)

type participant struct {
	id           int64
	pubAddr      string
	raftPubAddr  string
	seeds        map[string]bool
	tickDuration time.Duration

	client  *v2client
	peerHub *peerHub

	proposal    chan v2Proposal
	addNodeC    chan raft.Config
	removeNodeC chan raft.Config
	node        *v2Raft
	store.Store
	rh *raftHandler

	stopped bool
	mu      sync.Mutex
	stopc   chan struct{}

	*http.ServeMux
}

func newParticipant(id int64, pubAddr string, raftPubAddr string, seeds map[string]bool, client *v2client, peerHub *peerHub, tickDuration time.Duration) *participant {
	p := &participant{
		id:           id,
		pubAddr:      pubAddr,
		raftPubAddr:  raftPubAddr,
		seeds:        seeds,
		tickDuration: tickDuration,

		client:  client,
		peerHub: peerHub,

		proposal:    make(chan v2Proposal, maxBufferedProposal),
		addNodeC:    make(chan raft.Config, 1),
		removeNodeC: make(chan raft.Config, 1),
		node: &v2Raft{
			Node:   raft.New(id, defaultHeartbeat, defaultElection),
			result: make(map[wait]chan interface{}),
		},
		Store: store.New(),
		rh:    newRaftHandler(peerHub),

		stopc: make(chan struct{}),

		ServeMux: http.NewServeMux(),
	}

	p.Handle(v2Prefix+"/", handlerErr(p.serveValue))
	p.Handle(v2machinePrefix, handlerErr(p.serveMachines))
	p.Handle(v2peersPrefix, handlerErr(p.serveMachines))
	p.Handle(v2LeaderPrefix, handlerErr(p.serveLeader))
	p.Handle(v2StoreStatsPrefix, handlerErr(p.serveStoreStats))
	p.Handle(v2adminConfigPrefix, handlerErr(p.serveAdminConfig))
	p.Handle(v2adminMachinesPrefix, handlerErr(p.serveAdminMachines))
	return p
}

func (p *participant) run() int64 {
	if len(p.seeds) == 0 {
		log.Println("starting a bootstrap node")
		p.node.Campaign()
		p.node.Add(p.id, p.raftPubAddr, []byte(p.pubAddr))
		p.apply(p.node.Next())
	} else {
		log.Println("joining cluster via peers", p.seeds)
		p.join()
	}

	p.rh.start()
	defer p.rh.stop()

	node := p.node
	defer node.StopProposalWaiters()

	recv := p.rh.recv
	ticker := time.NewTicker(p.tickDuration)
	v2SyncTicker := time.NewTicker(time.Millisecond * 500)

	var proposal chan v2Proposal
	var addNodeC, removeNodeC chan raft.Config
	for {
		if node.HasLeader() {
			proposal = p.proposal
			addNodeC = p.addNodeC
			removeNodeC = p.removeNodeC
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
		case <-p.stopc:
			log.Printf("Participant %d stopped\n", p.id)
			return stopMode
		}
		p.apply(node.Next())
		p.send(node.Msgs())
		if node.IsRemoved() {
			log.Printf("Participant %d return\n", p.id)
			p.stop()
			return standbyMode
		}
	}
}

func (p *participant) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return
	}
	p.stopped = true
	close(p.stopc)
}

func (p *participant) raftHandler() http.Handler {
	return p.rh
}

func (p *participant) add(id int64, raftPubAddr string, pubAddr string) error {
	pp := path.Join(v2machineKVPrefix, fmt.Sprint(id))

	_, err := p.Get(pp, false, false)
	if err == nil {
		return nil
	}
	if v, ok := err.(*etcdErr.Error); !ok || v.ErrorCode != etcdErr.EcodeKeyNotFound {
		return err
	}

	w, err := p.Watch(pp, true, false, 0)
	if err != nil {
		log.Println("add error:", err)
		return tmpErr
	}

	select {
	case p.addNodeC <- raft.Config{NodeId: id, Addr: raftPubAddr, Context: []byte(pubAddr)}:
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
	case <-time.After(6 * defaultHeartbeat * p.tickDuration):
		w.Remove()
		log.Println("add error: wait timeout")
		return tmpErr
	case <-p.stopc:
		return stopErr
	}
}

func (p *participant) remove(id int64) error {
	pp := path.Join(v2machineKVPrefix, fmt.Sprint(id))

	v, err := p.Get(pp, false, false)
	if err != nil {
		return nil
	}

	select {
	case p.removeNodeC <- raft.Config{NodeId: id}:
	default:
		log.Println("unable to send out removeNode proposal")
		return tmpErr
	}

	// TODO(xiangli): do not need to watch if the
	// removal target is self
	w, err := p.Watch(pp, true, false, v.Index()+1)
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
	case <-time.After(6 * defaultHeartbeat * p.tickDuration):
		w.Remove()
		log.Println("remove error: wait timeout")
		return tmpErr
	case <-p.stopc:
		return stopErr
	}
}

func (p *participant) apply(ents []raft.Entry) {
	offset := p.node.Applied() - int64(len(ents)) + 1
	for i, ent := range ents {
		switch ent.Type {
		// expose raft entry type
		case raft.Normal:
			if len(ent.Data) == 0 {
				continue
			}
			p.v2apply(offset+int64(i), ent)
		case raft.AddNode:
			cfg := new(raft.Config)
			if err := json.Unmarshal(ent.Data, cfg); err != nil {
				log.Println(err)
				break
			}
			peer, err := p.peerHub.add(cfg.NodeId, cfg.Addr)
			if err != nil {
				log.Println(err)
				break
			}
			peer.participate()
			log.Printf("Add Node %x %v %v\n", cfg.NodeId, cfg.Addr, string(cfg.Context))
			pp := path.Join(v2machineKVPrefix, fmt.Sprint(cfg.NodeId))
			if _, err := p.Store.Set(pp, false, fmt.Sprintf("raft=%v&etcd=%v", cfg.Addr, string(cfg.Context)), store.Permanent); err == nil {
				p.seeds[cfg.Addr] = true
			}
		case raft.RemoveNode:
			cfg := new(raft.Config)
			if err := json.Unmarshal(ent.Data, cfg); err != nil {
				log.Println(err)
				break
			}
			log.Printf("Remove Node %x\n", cfg.NodeId)
			delete(p.seeds, p.fetchAddrFromStore(cfg.NodeId))
			peer, err := p.peerHub.peer(cfg.NodeId)
			if err != nil {
				log.Fatal("cannot get the added peer:", err)
			}
			peer.idle()
			pp := path.Join(v2machineKVPrefix, fmt.Sprint(cfg.NodeId))
			p.Store.Delete(pp, false, false)
		default:
			panic("unimplemented")
		}
	}
}

func (p *participant) send(msgs []raft.Message) {
	for i := range msgs {
		if err := p.peerHub.send(msgs[i]); err != nil {
			log.Println("send:", err)
		}
	}
}

func (p *participant) fetchAddrFromStore(nodeId int64) string {
	pp := path.Join(v2machineKVPrefix, fmt.Sprint(nodeId))
	if ev, err := p.Get(pp, false, false); err == nil {
		if m, err := url.ParseQuery(*ev.Node.Value); err == nil {
			return m["raft"][0]
		}
	}
	return ""
}

func (p *participant) join() {
	info := &context{
		MinVersion: store.MinVersion(),
		MaxVersion: store.MaxVersion(),
		ClientURL:  p.pubAddr,
		PeerURL:    p.raftPubAddr,
	}

	for {
		for seed := range p.seeds {
			if err := p.client.AddMachine(seed, fmt.Sprint(p.id), info); err == nil {
				return
			} else {
				log.Println(err)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Println("fail to join the cluster")
}
