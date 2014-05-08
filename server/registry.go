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
const RegistryKey = "/_etcd/machines"

// The Registry stores URL information for nodes.
type Registry struct {
	sync.Mutex
	store store.Store
	peers map[string]*node
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
		store: s,
		peers: make(map[string]*node),
	}
}

// Register adds a peer to the registry.
func (r *Registry) Register(name string, peerURL string, machURL string) error {
	// Write data to store.
	v := url.Values{}
	v.Set("raft", peerURL)
	v.Set("etcd", machURL)
	log.Debugf("Register: %s", name)
	if _, err := r.store.Create(path.Join(RegistryKey, name), false, v.Encode(), false, store.Permanent); err != nil {
		return err
	}

	r.Lock()
	defer r.Unlock()
	r.peers[name] = r.load(RegistryKey, name)
	return nil
}

// Unregister removes a peer from the registry.
func (r *Registry) Unregister(name string) error {
	// Remove the key from the store.
	log.Debugf("Unregister: %s", name)
	_, err := r.store.Delete(path.Join(RegistryKey, name), false, false)
	return err
}

// Count returns the number of peers in the cluster.
func (r *Registry) Count() int {
	e, err := r.store.Get(RegistryKey, false, false)
	if err != nil {
		return 0
	}
	return len(e.Node.Nodes)
}

// Exists checks if a peer with the given name exists.
func (r *Registry) Exists(name string) bool {
	e, err := r.store.Get(path.Join(RegistryKey, name), false, false)
	if err != nil {
		return false
	}
	return (e.Node != nil)
}

// Retrieves the client URL for a given node by name.
func (r *Registry) ClientURL(name string) (string, bool) {
	r.Lock()
	defer r.Unlock()
	return r.clientURL(RegistryKey, name)
}

func (r *Registry) clientURL(key, name string) (string, bool) {
	if r.peers[name] == nil {
		if peer := r.load(key, name); peer != nil {
			r.peers[name] = peer
		}
	}

	if peer := r.peers[name]; peer != nil {
		return peer.url, true
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
	return r.peerURL(RegistryKey, name)
}

func (r *Registry) peerURL(key, name string) (string, bool) {
	if r.peers[name] == nil {
		if peer := r.load(key, name); peer != nil {
			r.peers[name] = peer
		}
	}

	if peer := r.peers[name]; peer != nil {
		return peer.peerURL, true
	}

	return "", false
}

// UpdatePeerURL updates peer URL in registry
func (r *Registry) UpdatePeerURL(name string, peerURL string) error {
	machURL, _ := r.clientURL(RegistryKey, name)
	// Write data to store.
	v := url.Values{}
	v.Set("raft", peerURL)
	v.Set("etcd", machURL)
	log.Debugf("Update PeerURL: %s", name)
	if _, err := r.store.Update(path.Join(RegistryKey, name), v.Encode(), store.Permanent); err != nil {
		return err
	}

	r.Lock()
	defer r.Unlock()
	// Invalidate outdated cache.
	r.invalidate(name)
	return nil
}

func (r *Registry) name(key, name string) (string, bool) {
	return name, true
}

// Names returns a list of cached peer names.
func (r *Registry) Names() []string {
	names := r.urls(RegistryKey, "", "", r.name)
	sort.Sort(sort.StringSlice(names))
	return names
}

// Retrieves the Client URLs for all nodes.
func (r *Registry) ClientURLs(leaderName, selfName string) []string {
	return r.urls(RegistryKey, leaderName, selfName, r.clientURL)
}

// Retrieves the Peer URLs for all nodes.
func (r *Registry) PeerURLs(leaderName, selfName string) []string {
	return r.urls(RegistryKey, leaderName, selfName, r.peerURL)
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
