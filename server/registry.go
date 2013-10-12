package server

import (
    "fmt"
    "net/url"
    "path"
    "sync"

    "github.com/coreos/etcd/store"
)

// The location of the machine URL data.
const RegistryKey = "/_etcd/machines"

// The Registry stores URL information for nodes.
type Registry struct {
    sync.Mutex
    store *store.Store
    nodes map[string]*node
}

// The internal storage format of the registry.
type node struct {
    peerVersion string
    peerURL     string
    url     string
}

// Creates a new Registry.
func NewRegistry(s *store.Store) *Registry {
    return &Registry{
        store: s,
        nodes: make(map[string]*node),
    }
}

// Adds a node to the registry.
func (r *Registry) Register(name string, peerVersion string, peerURL string, url string, commitIndex uint64, term uint64) {
    r.Lock()
    defer r.Unlock()

    // Write data to store.
    key := path.Join(RegistryKey, name)
    value := fmt.Sprintf("raft=%s&etcd=%s&raftVersion=%s", peerURL, url, peerVersion)
    r.store.Create(key, value, false, false, store.Permanent, commitIndex, term)
}

// Removes a node from the registry.
func (r *Registry) Unregister(name string, commitIndex uint64, term uint64) error {
    r.Lock()
    defer r.Unlock()

    // Remove the key from the store.
    _, err := r.store.Delete(path.Join(RegistryKey, name), false, commitIndex, term)
    return err
}

// Returns the number of nodes in the cluster.
func (r *Registry) Count() int {
    e, err := r.store.Get(RegistryKey, false, false, 0, 0)
    if err != nil {
        return 0
    }
    return len(e.KVPairs)
}

// Retrieves the URL for a given node by name.
func (r *Registry) URL(name string) (string, bool) {
    r.Lock()
    defer r.Unlock()
    return r.url(name)
}

func (r *Registry) url(name string) (string, bool) {
    if r.nodes[name] == nil {
        r.load(name)
    }

    if node := r.nodes[name]; node != nil {
        return node.url, true
    }

    return "", false
}

// Retrieves the URLs for all nodes.
func (r *Registry) URLs() []string {
    r.Lock()
    defer r.Unlock()

    // Retrieve a list of all nodes.
    e, err := r.store.Get(RegistryKey, false, false, 0, 0)
    if err != nil {
        return make([]string, 0)
    }

    // Lookup the URL for each one.
    urls := make([]string, 0)
    for _, pair := range e.KVPairs {
        if url, ok := r.url(pair.Key); ok {
            urls = append(urls, url)
        }
    }

    return urls
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

// Retrieves the peer URLs for all nodes.
func (r *Registry) PeerURLs() []string {
    r.Lock()
    defer r.Unlock()

    // Retrieve a list of all nodes.
    e, err := r.store.Get(RegistryKey, false, false, 0, 0)
    if err != nil {
        return make([]string, 0)
    }

    // Lookup the URL for each one.
    urls := make([]string, 0)
    for _, pair := range e.KVPairs {
        if url, ok := r.peerURL(pair.Key); ok {
            urls = append(urls, url)
        }
    }

    return urls
}

// Loads the given node by name from the store into the cache.
func (r *Registry) load(name string) {
    if name == "" {
        return
    }

    // Retrieve from store.
    e, err := r.store.Get(path.Join(RegistryKey, name), false, false, 0, 0)
    if err != nil {
        return
    }

    // Parse as a query string.
    m, err := url.ParseQuery(e.Value)
    if err != nil {
        panic(fmt.Sprintf("Failed to parse machines entry: %s", name))
    }

    // Create node.
    r.nodes[name] = &node{
        url: m["etcd"][0],
        peerURL: m["raft"][0],
        peerVersion: m["raftVersion"][0],
    }
}
