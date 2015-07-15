// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"errors"
	"log"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/jonboulle/clockwork"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/pkg/types"
)

var (
	ErrInvalidURL           = errors.New("discovery: invalid URL")
	ErrBadSizeKey           = errors.New("discovery: size key is bad")
	ErrSizeNotFound         = errors.New("discovery: size key not found")
	ErrTokenNotFound        = errors.New("discovery: token not found")
	ErrDuplicateID          = errors.New("discovery: found duplicate id")
	ErrDuplicateName        = errors.New("discovery: found duplicate name")
	ErrFullCluster          = errors.New("discovery: cluster is full")
	ErrTooManyRetries       = errors.New("discovery: too many retries")
	ErrBadDiscoveryEndpoint = errors.New("discovery: bad discovery endpoint")
)

var (
	// Number of retries discovery will attempt before giving up and erroring out.
	nRetries = uint(math.MaxUint32)
)

func NewDiscoveryService(c Client, token string, id types.ID) DiscoveryService {
	return &discovery{
		cluster: token,
		c:       &httpKeysAPI{client: c},
		id:      id,
		clock:   clockwork.NewRealClock(),
	}
}

type DiscoveryService interface {
	JoinCluster(config string) (string, error)
	GetCluster() (string, error)
}

type discovery struct {
	cluster string
	id      types.ID
	c       KeysAPI
	retries uint

	clock clockwork.Clock
}

func (d *discovery) JoinCluster(config string) (string, error) {
	// fast path: if the cluster is full, return the error
	// do not need to register to the cluster in this case.
	if _, _, _, err := d.checkCluster(); err != nil {
		return "", err
	}

	if err := d.createSelf(config); err != nil {
		// Fails, even on a timeout, if createSelf times out.
		// TODO(barakmich): Retrying the same node might want to succeed here
		// (ie, createSelf should be idempotent for discovery).
		return "", err
	}

	nodes, size, index, err := d.checkCluster()
	if err != nil {
		return "", err
	}

	all, err := d.waitNodes(nodes, size, index)
	if err != nil {
		return "", err
	}

	return nodesToCluster(all, size)
}

func (d *discovery) GetCluster() (string, error) {
	nodes, size, index, err := d.checkCluster()
	if err != nil {
		if err == ErrFullCluster {
			return nodesToCluster(nodes, size)
		}
		return "", err
	}

	all, err := d.waitNodes(nodes, size, index)
	if err != nil {
		return "", err
	}
	return nodesToCluster(all, size)
}

func (d *discovery) createSelf(contents string) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRequestTimeout)
	resp, err := d.c.Create(ctx, d.selfKey(), contents)
	cancel()
	if err != nil {
		if eerr, ok := err.(Error); ok && eerr.Code == ErrorCodeNodeExist {
			return ErrDuplicateID
		}
		return err
	}

	// ensure self appears on the server we connected to
	w := d.c.Watcher(d.selfKey(), &WatcherOptions{AfterIndex: resp.Node.CreatedIndex - 1})
	_, err = w.Next(context.Background())
	return err
}

func (d *discovery) checkCluster() ([]*Node, int, uint64, error) {
	configKey := path.Join("/", d.cluster, "_config")
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRequestTimeout)
	// find cluster size
	resp, err := d.c.Get(ctx, path.Join(configKey, "size"), nil)
	cancel()
	if err != nil {
		if eerr, ok := err.(*Error); ok && eerr.Code == ErrorCodeKeyNotFound {
			return nil, 0, 0, ErrSizeNotFound
		}
		if err == ErrInvalidJSON {
			return nil, 0, 0, ErrBadDiscoveryEndpoint
		}
		if err == context.DeadlineExceeded {
			return d.checkClusterRetry()
		}
		return nil, 0, 0, err
	}
	size, err := strconv.Atoi(resp.Node.Value)
	if err != nil {
		return nil, 0, 0, ErrBadSizeKey
	}

	ctx, cancel = context.WithTimeout(context.Background(), DefaultRequestTimeout)
	resp, err = d.c.Get(ctx, d.cluster, nil)
	cancel()
	if err != nil {
		if err == context.DeadlineExceeded {
			return d.checkClusterRetry()
		}
		return nil, 0, 0, err
	}
	nodes := make([]*Node, 0)
	// append non-config keys to nodes
	for _, n := range resp.Node.Nodes {
		if !(path.Base(n.Key) == path.Base(configKey)) {
			nodes = append(nodes, n)
		}
	}

	snodes := sortableNodes{nodes}
	sort.Sort(snodes)

	// find self position
	for i := range nodes {
		if path.Base(nodes[i].Key) == path.Base(d.selfKey()) {
			break
		}
		if i >= size-1 {
			return nodes[:size], size, resp.Index, ErrFullCluster
		}
	}
	return nodes, size, resp.Index, nil
}

func (d *discovery) logAndBackoffForRetry(step string) {
	d.retries++
	retryTime := time.Second * (0x1 << d.retries)
	log.Printf("%s: connection to discovery service timed out, retrying in %s", step, retryTime)
	d.clock.Sleep(retryTime)
}

func (d *discovery) checkClusterRetry() ([]*Node, int, uint64, error) {
	if d.retries < nRetries {
		d.logAndBackoffForRetry("cluster status check")
		return d.checkCluster()
	}
	return nil, 0, 0, ErrTooManyRetries
}

func (d *discovery) waitNodesRetry() ([]*Node, error) {
	if d.retries < nRetries {
		d.logAndBackoffForRetry("waiting for other nodes")
		nodes, n, index, err := d.checkCluster()
		if err != nil {
			return nil, err
		}
		return d.waitNodes(nodes, n, index)
	}
	return nil, ErrTooManyRetries
}

func (d *discovery) waitNodes(nodes []*Node, size int, index uint64) ([]*Node, error) {
	if len(nodes) > size {
		nodes = nodes[:size]
	}
	// watch from the next index
	w := d.c.Watcher(d.cluster, &WatcherOptions{AfterIndex: index, Recursive: true})
	all := make([]*Node, len(nodes))
	copy(all, nodes)
	for _, n := range all {
		if path.Base(n.Key) == path.Base(d.selfKey()) {
			log.Printf("found self %s in the cluster", path.Base(d.selfKey()))
		} else {
			log.Printf("found peer %s in the cluster", path.Base(n.Key))
		}
	}

	// wait for others
	for len(all) < size {
		log.Printf("found %d peer(s), waiting for %d more", len(all), size-len(all))
		resp, err := w.Next(context.Background())
		if err != nil {
			if err == context.DeadlineExceeded {
				return d.waitNodesRetry()
			}
			return nil, err
		}
		log.Printf("found peer %s in the cluster", path.Base(resp.Node.Key))
		all = append(all, resp.Node)
	}
	log.Printf("found %d needed peer(s)", len(all))
	return all, nil
}

func (d *discovery) selfKey() string {
	return path.Join("/", d.cluster, d.id.String())
}

func nodesToCluster(ns []*Node, size int) (string, error) {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = n.Value
	}
	us := strings.Join(s, ",")
	m, err := types.NewURLsMap(us)
	if err != nil {
		return us, ErrInvalidURL
	}
	if m.Len() != size {
		return us, ErrDuplicateName
	}
	return us, nil
}

type sortableNodes struct{ Nodes []*Node }

func (ns sortableNodes) Len() int { return len(ns.Nodes) }
func (ns sortableNodes) Less(i, j int) bool {
	return ns.Nodes[i].CreatedIndex < ns.Nodes[j].CreatedIndex
}
func (ns sortableNodes) Swap(i, j int) { ns.Nodes[i], ns.Nodes[j] = ns.Nodes[j], ns.Nodes[i] }
