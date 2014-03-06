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

// The location of the proxy URL data.
const RegistryProxyKey = "/_etcd/proxies"

// The Registry stores URL information for nodes.
type Registry struct {
	sync.Mutex
	store   store.Store
	peers   map[string]*node
	proxies map[string]*node
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
		store:   s,
		peers:   make(map[string]*node),
		proxies: make(map[string]*node),
	}
}

// Peers returns a list of cached peer names.
func (r *Registry) Peers() []string {
	names := make([]string, 0, len(r.peers))
	for name, _ := range r.peers {
		names = append(names, name)
	}
	sort.Sort(sort.StringSlice(names))
	return names
}

// Proxies returns a list of cached proxy names.
func (r *Registry) Proxies() []string {
	names := make([]string, 0, len(r.proxies))
	for name, _ := range r.proxies {
		names = append(names, name)
	}
	sort.Sort(sort.StringSlice(names))
	return names
}

// RegisterPeer adds a peer to the registry.
func (r *Registry) RegisterPeer(name string, peerURL string, machURL string) error {
	// TODO(benbjohnson): Disallow peers that are already proxies.
	return r.register(RegistryPeerKey, name, peerURL, machURL)
}

// RegisterProxy adds a proxy to the registry.
func (r *Registry) RegisterProxy(name string, peerURL string, machURL string) error {
	// TODO(benbjohnson): Disallow proxies that are already peers.
	if err := r.register(RegistryProxyKey, name, peerURL, machURL); err != nil {
		return err
	}
	r.proxies[name] = r.load(RegistryProxyKey, name)
	return nil
}

func (r *Registry) register(key, name string, peerURL string, machURL string) error {
	r.Lock()
	defer r.Unlock()

	// Write data to store.
	v := url.Values{}
	v.Set("raft", peerURL)
	v.Set("etcd", machURL)
	_, err := r.store.Create(path.Join(key, name), false, v.Encode(), false, store.Permanent)
	log.Debugf("Register: %s", name)
	return err
}

// UnregisterPeer removes a peer from the registry.
func (r *Registry) UnregisterPeer(name string) error {
	return r.unregister(RegistryPeerKey, name)
}

// UnregisterProxy removes a proxy from the registry.
func (r *Registry) UnregisterProxy(name string) error {
	return r.unregister(RegistryProxyKey, name)
}

func (r *Registry) unregister(key, name string) error {
	r.Lock()
	defer r.Unlock()

	// Remove the key from the store.
	_, err := r.store.Delete(path.Join(key, name), false, false)
	log.Debugf("Unregister: %s", name)
	return err
}

// PeerCount returns the number of peers in the cluster.
func (r *Registry) PeerCount() int {
	return r.count(RegistryPeerKey)
}

// ProxyCount returns the number of proxies in the cluster.
func (r *Registry) ProxyCount() int {
	return r.count(RegistryProxyKey)
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

// ProxyExists checks if a proxy with the given name exists.
func (r *Registry) ProxyExists(name string) bool {
	return r.exists(RegistryProxyKey, name)
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

// Retrieves the client URL for a given proxy by name.
func (r *Registry) ProxyClientURL(name string) (string, bool) {
	r.Lock()
	defer r.Unlock()
	return r.proxyClientURL(RegistryProxyKey, name)
}

func (r *Registry) proxyClientURL(key, name string) (string, bool) {
	if r.proxies[name] == nil {
		if node := r.load(key, name); node != nil {
			r.proxies[name] = node
		}
	}
	if node := r.proxies[name]; node != nil {
		return node.url, true
	}
	return "", false
}

// Retrieves the peer URL for a given proxy by name.
func (r *Registry) ProxyPeerURL(name string) (string, bool) {
	r.Lock()
	defer r.Unlock()
	return r.proxyPeerURL(RegistryProxyKey, name)
}

func (r *Registry) proxyPeerURL(key, name string) (string, bool) {
	if r.proxies[name] == nil {
		if node := r.load(key, name); node != nil {
			r.proxies[name] = node
		}
	}
	if node := r.proxies[name]; node != nil {
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

	log.Infof("URLs: %s: %s / %s (%s)", key, leaderName, selfName, strings.Join(urls, ","))

	return urls
}

// Removes a node from the cache.
func (r *Registry) Invalidate(name string) {
	delete(r.peers, name)
	delete(r.proxies, name)
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
