package discovery

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
)

var (
	ErrInvalidURL    = errors.New("discovery: invalid URL")
	ErrBadSizeKey    = errors.New("discovery: size key is bad")
	ErrSizeNotFound  = errors.New("discovery: size key not found")
	ErrTokenNotFound = errors.New("discovery: token not found")
	ErrDuplicateID   = errors.New("discovery: found duplicate id")
	ErrFullCluster   = errors.New("discovery: cluster is full")
)

type discovery struct {
	cluster string
	id      int64
	ctx     []byte
	c       client.Client
}

func (d *discovery) discover() (*etcdhttp.Peers, error) {
	// fast path: if the cluster is full, returns the error
	// do not need to register itself to the cluster in this
	// case.
	if _, _, err := d.checkCluster(); err != nil {
		return nil, err
	}

	if err := d.createSelf(); err != nil {
		return nil, err
	}

	nodes, size, err := d.checkCluster()
	if err != nil {
		return nil, err
	}

	all, err := d.waitNodes(nodes, size)
	if err != nil {
		return nil, err
	}

	return nodesToPeers(all)
}

func (d *discovery) createSelf() error {
	// create self key
	resp, err := d.c.Create(d.selfKey(), string(d.ctx), 0)
	if err != nil {
		return err
	}

	// ensure self appears on the server we connected to
	w := d.c.Watch(d.selfKey(), resp.Node.CreatedIndex)
	if _, err = w.Next(); err != nil {
		return err
	}
	return nil
}

func (d *discovery) checkCluster() (client.Nodes, int, error) {
	configKey := path.Join("/", d.cluster, "config")
	// find cluster size
	resp, err := d.c.Get(path.Join(configKey, "size"))
	if err != nil {
		if err == client.ErrKeyNoExist {
			return nil, 0, ErrSizeNotFound
		}
		return nil, 0, err
	}
	size, err := strconv.Atoi(resp.Node.Value)
	if err != nil {
		return nil, 0, ErrBadSizeKey
	}

	resp, err = d.c.Get(d.cluster)
	if err != nil {
		return nil, 0, err
	}
	nodes := make(client.Nodes, 0)
	// append non-config keys to nodes
	for _, n := range resp.Node.Nodes {
		if !strings.HasPrefix(n.Key, configKey) {
			nodes = append(nodes, n)
		}
	}

	snodes := SortableNodes{nodes}
	sort.Sort(snodes)

	// find self position
	for i := range nodes {
		if nodes[i].Key == d.selfKey() {
			break
		}
		if i >= size-1 {
			return nil, size, ErrFullCluster
		}
	}
	return nodes, size, nil
}

func (d *discovery) waitNodes(nodes client.Nodes, size int) (client.Nodes, error) {
	if len(nodes) > size {
		nodes = nodes[:size]
	}
	w := d.c.RecursiveWatch(d.cluster, nodes[len(nodes)-1].ModifiedIndex)
	all := make(client.Nodes, len(nodes))
	copy(all, nodes)
	// wait for others
	for len(all) < size {
		resp, err := w.Next()
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Node)
	}
	return all, nil
}

func (d *discovery) selfKey() string {
	return path.Join("/", d.cluster, fmt.Sprintf("%d", d.id))
}

func nodesToPeers(ns client.Nodes) (*etcdhttp.Peers, error) {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = n.Value
	}

	var peers etcdhttp.Peers
	if err := peers.Set(strings.Join(s, "&")); err != nil {
		return nil, err
	}
	return &peers, nil
}

type SortableNodes struct{ client.Nodes }

func (ns SortableNodes) Len() int { return len(ns.Nodes) }
func (ns SortableNodes) Less(i, j int) bool {
	return ns.Nodes[i].CreatedIndex < ns.Nodes[j].CreatedIndex
}
func (ns SortableNodes) Swap(i, j int) { ns.Nodes[i], ns.Nodes[j] = ns.Nodes[j], ns.Nodes[i] }
