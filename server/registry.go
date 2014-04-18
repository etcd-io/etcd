package server

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/store"
)

// The location of the peer URL data.
const RegistryPeerKey = "/_etcd/machines"

// The location of the standby URL data.
const RegistryStandbyKey = "/_etcd/standbys"

// The Registry stores URL information for nodes.
type Registry struct {
	sync.Mutex
	store    store.Store
	peers    map[string]*node
	standbys map[string]*node
}

// The internal storage format of the registry.
type node struct {
	peerVersion string
	peerURL     string
	url         string
}

// Creates a new Registry.
func NewRegistry(s store.Store) *Registry {
	return &Registry{
		store:    s,
		peers:    make(map[string]*node),
		standbys: make(map[string]*node),
	}
}

// Peers returns a list of cached peer names.
func (r *Registry) Peers() []string {
	r.Lock()
	defer r.Unlock()

	names := make([]string, 0, len(r.peers))
	for name := range r.peers {
		names = append(names, name)
	}
	sort.Sort(sort.StringSlice(names))
	return names
}

// Standbys returns a list of cached standby names.
func (r *Registry) Standbys() []string {
	r.Lock()
	defer r.Unlock()

	names := make([]string, 0, len(r.standbys))
	for name := range r.standbys {
		names = append(names, name)
	}
	sort.Sort(sort.StringSlice(names))
	return names
}

// RegisterPeer adds a peer to the registry.
func (r *Registry) RegisterPeer(name string, peerURL string, machURL string) error {
	if err := r.register(RegistryPeerKey, name, peerURL, machURL); err != nil {
		return err
	}

	r.Lock()
	defer r.Unlock()
	r.peers[name] = r.load(RegistryPeerKey, name)
	return nil
}

// RegisterStandby adds a standby to the registry.
func (r *Registry) RegisterStandby(name string, peerURL string, machURL string) error {
	if err := r.register(RegistryStandbyKey, name, peerURL, machURL); err != nil {
		return err
	}

	r.Lock()
	defer r.Unlock()
	r.standbys[name] = r.load(RegistryStandbyKey, name)
	return nil
}

func (r *Registry) register(key, name string, peerURL string, machURL string) error {
	// Write data to store.
	v := url.Values{}
	v.Set("raft", peerURL)
	v.Set("etcd", machURL)
	_, err := r.store.Create(path.Join(key, name), false, v.Encode(), false, store.Permanent)
	log.Debugf("Register: %s", name)
	return err
}

// UpdatePeerURL updates peer URL in registry
func (r *Registry) UpdatePeerURL(name string, peerURL string) error {
	r.Lock()
	defer r.Unlock()

	machURL, _ := r.clientURL(RegistryPeerKey, name)
	// Write data to store.
	key := path.Join(RegistryPeerKey, name)
	v := url.Values{}
	v.Set("raft", peerURL)
	v.Set("etcd", machURL)
	_, err := r.store.Update(key, v.Encode(), store.Permanent)

	// Invalidate outdated cache.
	r.invalidate(name)
	log.Debugf("Update PeerURL: %s", name)
	return err
}

// UnregisterPeer removes a peer from the registry.
func (r *Registry) UnregisterPeer(name string) error {
	return r.unregister(RegistryPeerKey, name)
}

// UnregisterStandby removes a standby from the registry.
func (r *Registry) UnregisterStandby(name string) error {
	return r.unregister(RegistryStandbyKey, name)
}

func (r *Registry) unregister(key, name string) error {
	// Remove the key from the store.
	_, err := r.store.Delete(path.Join(key, name), false, false)
	log.Debugf("Unregister: %s", name)
	return err
}

// PeerCount returns the number of peers in the cluster.
func (r *Registry) PeerCount() int {
	return r.count(RegistryPeerKey)
}

// StandbyCount returns the number of standbys in the cluster.
func (r *Registry) StandbyCount() int {
	return r.count(RegistryStandbyKey)
}

// Returns the number of nodes in the cluster.
func (r *Registry) count(key string) int {
	e, err := r.store.Get(key, false, false)
	if err != nil {
		return 0
	}
	return len(e.Node.Nodes)
}

// PeerExists checks if a peer with the given name exists.
func (r *Registry) PeerExists(name string) bool {
	return r.exists(RegistryPeerKey, name)
}

// StandbyExists checks if a standby with the given name exists.
func (r *Registry) StandbyExists(name string) bool {
	return r.exists(RegistryStandbyKey, name)
}

func (r *Registry) exists(key, name string) bool {
	e, err := r.store.Get(path.Join(key, name), false, false)
	if err != nil {
		return false
	}
	return (e.Node != nil)
}

// Retrieves the client URL for a given node by name.
func (r *Registry) ClientURL(name string) (string, bool) {
	r.Lock()
	defer r.Unlock()
	return r.clientURL(RegistryPeerKey, name)
}

func (r *Registry) clientURL(key, name string) (string, bool) {
	if r.peers[name] == nil {
		if node := r.load(key, name); node != nil {
			r.peers[name] = node
		}
	}

	if node := r.peers[name]; node != nil {
		return node.url, true
	}

	return "", false
}

// TODO(yichengq): have all of the code use a full URL with scheme
// and remove this method
// PeerHost retrieves the host part of peer URL for a given node by name.
func (r *Registry) PeerHost(name string) (string, bool) {
	rawurl, ok := r.PeerURL(name)
	if ok {
		u, _ := url.Parse(rawurl)
		return u.Host, ok
	}
	return rawurl, ok
}

// Retrieves the peer URL for a given node by name.
func (r *Registry) PeerURL(name string) (string, bool) {
	r.Lock()
	defer r.Unlock()
	return r.peerURL(RegistryPeerKey, name)
}

func (r *Registry) peerURL(key, name string) (string, bool) {
	if r.peers[name] == nil {
		if node := r.load(key, name); node != nil {
			r.peers[name] = node
		}
	}

	if node := r.peers[name]; node != nil {
		return node.peerURL, true
	}

	return "", false
}

// Retrieves the client URL for a given standby by name.
func (r *Registry) StandbyClientURL(name string) (string, bool) {
	r.Lock()
	defer r.Unlock()
	return r.standbyClientURL(RegistryStandbyKey, name)
}

func (r *Registry) standbyClientURL(key, name string) (string, bool) {
	if r.standbys[name] == nil {
		if node := r.load(key, name); node != nil {
			r.standbys[name] = node
		}
	}
	if node := r.standbys[name]; node != nil {
		return node.url, true
	}
	return "", false
}

// Retrieves the peer URL for a given standby by name.
func (r *Registry) StandbyPeerURL(name string) (string, bool) {
	r.Lock()
	defer r.Unlock()
	return r.standbyPeerURL(RegistryStandbyKey, name)
}

func (r *Registry) standbyPeerURL(key, name string) (string, bool) {
	if r.standbys[name] == nil {
		if node := r.load(key, name); node != nil {
			r.standbys[name] = node
		}
	}
	if node := r.standbys[name]; node != nil {
		return node.peerURL, true
	}
	return "", false
}

// Retrieves the Client URLs for all nodes.
func (r *Registry) ClientURLs(leaderName, selfName string) []string {
	return r.urls(RegistryPeerKey, leaderName, selfName, r.clientURL)
}

// Retrieves the Peer URLs for all nodes.
func (r *Registry) PeerURLs(leaderName, selfName string) []string {
	return r.urls(RegistryPeerKey, leaderName, selfName, r.peerURL)
}

// Retrieves the URLs for all nodes using url function.
func (r *Registry) urls(key, leaderName, selfName string, url func(key, name string) (string, bool)) []string {
	r.Lock()
	defer r.Unlock()

	// Build list including the leader and self.
	urls := make([]string, 0)
	if url, _ := url(key, leaderName); len(url) > 0 {
		urls = append(urls, url)
	}

	// Retrieve a list of all nodes.
	if e, _ := r.store.Get(key, false, false); e != nil {
		// Lookup the URL for each one.
		for _, pair := range e.Node.Nodes {
			_, name := filepath.Split(pair.Key)
			if url, _ := url(key, name); len(url) > 0 && name != leaderName {
				urls = append(urls, url)
			}
		}
	}

	log.Debugf("URLs: %s: %s / %s (%s)", key, leaderName, selfName, strings.Join(urls, ","))
	return urls
}

// Removes a node from the cache.
func (r *Registry) Invalidate(name string) {
	r.Lock()
	defer r.Unlock()
	r.invalidate(name)
}

func (r *Registry) invalidate(name string) {
	delete(r.peers, name)
	delete(r.standbys, name)
}

// Loads the given node by name from the store into the cache.
func (r *Registry) load(key, name string) *node {
	if name == "" {
		return nil
	}

	// Retrieve from store.
	e, err := r.store.Get(path.Join(key, name), false, false)
	if err != nil {
		return nil
	}

	// Parse as a query string.
	m, err := url.ParseQuery(*e.Node.Value)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse peers entry: %s", name))
	}

	// Create node.
	return &node{
		url:     m["etcd"][0],
		peerURL: m["raft"][0],
	}
}
