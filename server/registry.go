package server

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
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
	nodes map[string]*node
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
		nodes: make(map[string]*node),
	}
}

// Adds a node to the registry.
func (r *Registry) Register(name string, peerURL string, url string) error {
	r.Lock()
	defer r.Unlock()

	// Write data to store.
	key := path.Join(RegistryKey, name)
	value := fmt.Sprintf("raft=%s&etcd=%s", peerURL, url)
	_, err := r.store.Create(key, value, false, store.Permanent)
	log.Debugf("Register: %s", name)
	return err
}

// Removes a node from the registry.
func (r *Registry) Unregister(name string) error {
	r.Lock()
	defer r.Unlock()

	// Remove from cache.
	// delete(r.nodes, name)

	// Remove the key from the store.
	_, err := r.store.Delete(path.Join(RegistryKey, name), false)
	log.Debugf("Unregister: %s", name)
	return err
}

// Returns the number of nodes in the cluster.
func (r *Registry) Count() int {
	e, err := r.store.Get(RegistryKey, false, false)
	if err != nil {
		return 0
	}
	return len(e.Node.Nodes)
}

// Retrieves the client URL for a given node by name.
func (r *Registry) ClientURL(name string) (string, bool) {
	r.Lock()
	defer r.Unlock()
	return r.clientURL(name)
}

func (r *Registry) clientURL(name string) (string, bool) {
	if r.nodes[name] == nil {
		r.load(name)
	}

	if node := r.nodes[name]; node != nil {
		return node.url, true
	}

	return "", false
}

// Retrieves the peer URL for a given node by name.
func (r *Registry) PeerURL(name string) (string, bool) {
	r.Lock()
	defer r.Unlock()
	return r.peerURL(name)
}

func (r *Registry) peerURL(name string) (string, bool) {
	if r.nodes[name] == nil {
		r.load(name)
	}

	if node := r.nodes[name]; node != nil {
		return node.peerURL, true
	}

	return "", false
}

// Retrieves the Client URLs for all nodes.
func (r *Registry) ClientURLs(leaderName, selfName string) []string {
	return r.urls(leaderName, selfName, r.clientURL)
}

// Retrieves the Peer URLs for all nodes.
func (r *Registry) PeerURLs(leaderName, selfName string) []string {
	return r.urls(leaderName, selfName, r.peerURL)
}

// Retrieves the URLs for all nodes using url function.
func (r *Registry) urls(leaderName, selfName string, url func(name string) (string, bool)) []string {
	r.Lock()
	defer r.Unlock()

	// Build list including the leader and self.
	urls := make([]string, 0)
	if url, _ := url(leaderName); len(url) > 0 {
		urls = append(urls, url)
	}

	// Retrieve a list of all nodes.
	if e, _ := r.store.Get(RegistryKey, false, false); e != nil {
		// Lookup the URL for each one.
		for _, pair := range e.Node.Nodes {
			_, name := filepath.Split(pair.Key)
			if url, _ := url(name); len(url) > 0 && name != leaderName {
				urls = append(urls, url)
			}
		}
	}

	log.Infof("URLs: %s / %s (%s)", leaderName, selfName, strings.Join(urls, ","))

	return urls
}

// Removes a node from the cache.
func (r *Registry) Invalidate(name string) {
	delete(r.nodes, name)
}

// Loads the given node by name from the store into the cache.
func (r *Registry) load(name string) {
	if name == "" {
		return
	}

	// Retrieve from store.
	e, err := r.store.Get(path.Join(RegistryKey, name), false, false)
	if err != nil {
		return
	}

	// Parse as a query string.
	m, err := url.ParseQuery(e.Node.Value)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse peers entry: %s", name))
	}

	// Create node.
	r.nodes[name] = &node{
		url:     m["etcd"][0],
		peerURL: m["raft"][0],
	}
}
