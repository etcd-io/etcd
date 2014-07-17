package etcd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync"

	"github.com/coreos/etcd/raft"
)

var (
	errUnknownPeer = errors.New("unknown peer")
)

type peerGetter interface {
	peer(id int64) (*peer, error)
}

type peerHub struct {
	mu      sync.RWMutex
	stopped bool
	seeds   map[string]bool
	peers   map[int64]*peer
	c       *http.Client
}

func newPeerHub(seeds []string, c *http.Client) *peerHub {
	h := &peerHub{
		peers: make(map[int64]*peer),
		seeds: make(map[string]bool),
		c:     c,
	}
	for _, seed := range seeds {
		h.seeds[seed] = true
	}
	return h
}

func (h *peerHub) stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopped = true
	for _, p := range h.peers {
		p.stop()
	}
	tr := h.c.Transport.(*http.Transport)
	tr.CloseIdleConnections()
}

func (h *peerHub) peer(id int64) (*peer, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return nil, fmt.Errorf("peerHub stopped")
	}
	if p, ok := h.peers[id]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("peer %d not found", id)
}

func (h *peerHub) add(id int64, rawurl string) (*peer, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	u.Path = raftPrefix

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return nil, fmt.Errorf("peerHub stopped")
	}
	h.peers[id] = newPeer(u.String(), h.c)
	return h.peers[id], nil
}

func (h *peerHub) send(msg raft.Message) error {
	if p, err := h.fetch(msg.To); err == nil {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		return p.send(data)
	}
	return errUnknownPeer
}

func (h *peerHub) fetch(nodeId int64) (*peer, error) {
	if p, err := h.peer(nodeId); err == nil {
		return p, nil
	}
	for seed := range h.seeds {
		if p, err := h.seedFetch(seed, nodeId); err == nil {
			return p, nil
		}
	}
	return nil, fmt.Errorf("cannot fetch the address of node %d", nodeId)
}

func (h *peerHub) seedFetch(seedurl string, id int64) (*peer, error) {
	u, err := url.Parse(seedurl)
	if err != nil {
		return nil, fmt.Errorf("cannot parse the url of the given seed")
	}

	u.Path = path.Join("/raft/cfg", fmt.Sprint(id))
	resp, err := h.c.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("cannot reach %v", u)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot find node %d via %s", id, seedurl)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %v", u)
	}

	return h.add(id, string(b))
}
