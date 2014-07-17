package etcd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync"
)

type peerGetter interface {
	peer(id int64) (*peer, error)
}

type peerHub struct {
	mu    sync.RWMutex
	peers map[int64]*peer
	c     *http.Client
}

func newPeerHub(c *http.Client) *peerHub {
	h := &peerHub{
		peers: make(map[int64]*peer),
		c:     c,
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

func (h *peerHub) fetch(seedurl string, id int64) error {
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
		return errUnknownNode
	}
	return p.send(data)
}
