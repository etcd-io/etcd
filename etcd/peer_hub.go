package etcd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync"
)

var (
	errUnknownPeer = errors.New("unknown peer")
)

type peerGetter interface {
	peer(id int64) (*peer, error)
}

type peerHub struct {
	mu    sync.RWMutex
	seeds map[string]bool
	peers map[int64]*peer
	c     *http.Client
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
	for _, p := range h.peers {
		p.stop()
	}
	tr := h.c.Transport.(*http.Transport)
	tr.CloseIdleConnections()
}

func (h *peerHub) peer(id int64) (*peer, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if p, ok := h.peers[id]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("peer %d not found", id)
}

func (h *peerHub) add(id int64, rawurl string) error {
	u, err := url.Parse(rawurl)
	if err != nil {
		return err
	}
	u.Path = raftPrefix

	h.mu.Lock()
	defer h.mu.Unlock()
	h.peers[id] = newPeer(u.String(), h.c)
	return nil
}

func (h *peerHub) send(nodeId int64, data []byte) error {
	h.mu.RLock()
	p := h.peers[nodeId]
	h.mu.RUnlock()

	if p == nil {
		err := h.fetch(nodeId)
		if err != nil {
			return errUnknownPeer
		}
	}

	h.mu.RLock()
	p = h.peers[nodeId]
	h.mu.RUnlock()
	return p.send(data)
}

func (h *peerHub) fetch(nodeId int64) error {
	for seed := range h.seeds {
		if err := h.seedFetch(seed, nodeId); err == nil {
			return nil
		}
	}
	return fmt.Errorf("cannot fetch the address of node %d", nodeId)
}

func (h *peerHub) seedFetch(seedurl string, id int64) error {
	if _, err := h.peer(id); err == nil {
		return nil
	}

	u, err := url.Parse(seedurl)
	if err != nil {
		return fmt.Errorf("cannot parse the url of the given seed")
	}

	u.Path = path.Join("/raft/cfg", fmt.Sprint(id))
	resp, err := h.c.Get(u.String())
	if err != nil {
		return fmt.Errorf("cannot reach %v", u)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cannot find node %d via %s", id, seedurl)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cannot reach %v", u)
	}

	if err := h.add(id, string(b)); err != nil {
		return fmt.Errorf("cannot parse the url of node %d: %v", id, err)
	}
	return nil
}
