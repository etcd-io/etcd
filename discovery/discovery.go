/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package discovery

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/jonboulle/clockwork"
	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/types"
)

var (
	ErrInvalidURL     = errors.New("discovery: invalid URL")
	ErrBadSizeKey     = errors.New("discovery: size key is bad")
	ErrSizeNotFound   = errors.New("discovery: size key not found")
	ErrTokenNotFound  = errors.New("discovery: token not found")
	ErrDuplicateID    = errors.New("discovery: found duplicate id")
	ErrFullCluster    = errors.New("discovery: cluster is full")
	ErrTooManyRetries = errors.New("discovery: too many retries")
)

const (
	// Environment variable used to configure an HTTP proxy for discovery
	DiscoveryProxyEnv = "ETCD_DISCOVERY_PROXY"
	// Number of retries discovery will attempt before giving up and erroring out.
	nRetries = uint(3)
)

type Discoverer interface {
	Discover() (string, error)
}

type discovery struct {
	cluster string
	id      types.ID
	config  string
	c       client.KeysAPI
	retries uint
	url     *url.URL

	clock clockwork.Clock
}

// proxyFuncFromEnv builds a proxy function if the appropriate environment
// variable is set. It performs basic sanitization of the environment variable
// and returns any error encountered.
func proxyFuncFromEnv() (func(*http.Request) (*url.URL, error), error) {
	proxy := os.Getenv(DiscoveryProxyEnv)
	if proxy == "" {
		return nil, nil
	}
	// Do a small amount of URL sanitization to help the user
	// Derived from net/http.ProxyFromEnvironment
	proxyURL, err := url.Parse(proxy)
	if err != nil || !strings.HasPrefix(proxyURL.Scheme, "http") {
		// proxy was bogus. Try prepending "http://" to it and
		// see if that parses correctly. If not, we ignore the
		// error and complain about the original one
		var err2 error
		proxyURL, err2 = url.Parse("http://" + proxy)
		if err2 == nil {
			err = nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("invalid proxy address %q: %v", proxy, err)
	}

	log.Printf("discovery: using proxy %q", proxyURL.String())
	return http.ProxyURL(proxyURL), nil
}

func New(durl string, id types.ID, config string) (Discoverer, error) {
	u, err := url.Parse(durl)
	if err != nil {
		return nil, err
	}
	token := u.Path
	u.Path = ""
	pf, err := proxyFuncFromEnv()
	if err != nil {
		return nil, err
	}
	c, err := client.NewHTTPClient(&http.Transport{Proxy: pf}, []string{u.String()})
	if err != nil {
		return nil, err
	}
	dc := client.NewDiscoveryKeysAPI(c, client.DefaultRequestTimeout)
	return &discovery{
		cluster: token,
		id:      id,
		config:  config,
		c:       dc,
		url:     u,
		clock:   clockwork.NewRealClock(),
	}, nil
}

func (d *discovery) Discover() (string, error) {
	// fast path: if the cluster is full, returns the error
	// do not need to register itself to the cluster in this
	// case.
	if _, _, err := d.checkCluster(); err != nil {
		return "", err
	}

	if err := d.createSelf(); err != nil {
		// Fails, even on a timeout, if createSelf times out.
		// TODO(barakmich): Retrying the same node might want to succeed here
		// (ie, createSelf should be idempotent for discovery).
		return "", err
	}

	nodes, size, err := d.checkCluster()
	if err != nil {
		return "", err
	}

	all, err := d.waitNodes(nodes, size)
	if err != nil {
		return "", err
	}

	return nodesToCluster(all), nil
}

func (d *discovery) createSelf() error {
	resp, err := d.c.Create(d.selfKey(), d.config, -1)
	if err != nil {
		return err
	}

	// ensure self appears on the server we connected to
	w := d.c.Watch(d.selfKey(), resp.Node.CreatedIndex)
	_, err = w.Next()
	return err
}

func (d *discovery) checkCluster() (client.Nodes, int, error) {
	configKey := path.Join("/", d.cluster, "_config")
	// find cluster size
	resp, err := d.c.Get(path.Join(configKey, "size"))
	if err != nil {
		if err == client.ErrKeyNoExist {
			return nil, 0, ErrSizeNotFound
		}
		if err == client.ErrTimeout {
			return d.checkClusterRetry()
		}
		return nil, 0, err
	}
	size, err := strconv.Atoi(resp.Node.Value)
	if err != nil {
		return nil, 0, ErrBadSizeKey
	}

	resp, err = d.c.Get(d.cluster)
	if err != nil {
		if err == client.ErrTimeout {
			return d.checkClusterRetry()
		}
		return nil, 0, err
	}
	nodes := make(client.Nodes, 0)
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
			return nil, size, ErrFullCluster
		}
	}
	return nodes, size, nil
}

func (d *discovery) logAndBackoffForRetry(step string) {
	d.retries++
	retryTime := time.Second * (0x1 << d.retries)
	log.Println("discovery: during", step, "connection to", d.url, "timed out, retrying in", retryTime)
	d.clock.Sleep(retryTime)
}

func (d *discovery) checkClusterRetry() (client.Nodes, int, error) {
	if d.retries < nRetries {
		d.logAndBackoffForRetry("cluster status check")
		return d.checkCluster()
	}
	return nil, 0, ErrTooManyRetries
}

func (d *discovery) waitNodesRetry() (client.Nodes, error) {
	if d.retries < nRetries {
		d.logAndBackoffForRetry("waiting for other nodes")
		nodes, n, err := d.checkCluster()
		if err != nil {
			return nil, err
		}
		return d.waitNodes(nodes, n)
	}
	return nil, ErrTooManyRetries
}

func (d *discovery) waitNodes(nodes client.Nodes, size int) (client.Nodes, error) {
	if len(nodes) > size {
		nodes = nodes[:size]
	}
	w := d.c.RecursiveWatch(d.cluster, nodes[len(nodes)-1].ModifiedIndex+1)
	all := make(client.Nodes, len(nodes))
	copy(all, nodes)
	for _, n := range all {
		if path.Base(n.Key) == path.Base(d.selfKey()) {
			log.Printf("discovery: found self %s in the cluster", path.Base(d.selfKey()))
		} else {
			log.Printf("discovery: found peer %s in the cluster", path.Base(n.Key))
		}
	}

	// wait for others
	for len(all) < size {
		log.Printf("discovery: found %d peer(s), waiting for %d more", len(all), size-len(all))
		resp, err := w.Next()
		if err != nil {
			if err == client.ErrTimeout {
				return d.waitNodesRetry()
			}
			return nil, err
		}
		log.Printf("discovery: found peer %s in the cluster", path.Base(resp.Node.Key))
		all = append(all, resp.Node)
	}
	log.Printf("discovery: found %d needed peer(s)", len(all))
	return all, nil
}

func (d *discovery) selfKey() string {
	return path.Join("/", d.cluster, d.id.String())
}

func nodesToCluster(ns client.Nodes) string {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = n.Value
	}
	return strings.Join(s, ",")
}

type sortableNodes struct{ client.Nodes }

func (ns sortableNodes) Len() int { return len(ns.Nodes) }
func (ns sortableNodes) Less(i, j int) bool {
	return ns.Nodes[i].CreatedIndex < ns.Nodes[j].CreatedIndex
}
func (ns sortableNodes) Swap(i, j int) { ns.Nodes[i], ns.Nodes[j] = ns.Nodes[j], ns.Nodes[i] }
