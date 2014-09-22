package etcdhttp

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Cluster contains a mapping of node IDs to a list of hostnames/IP addresses
type Cluster map[int64][]string

// addScheme adds the protocol prefix to a string; currently only HTTP
// TODO: improve this when implementing TLS
func addScheme(addr string) string {
	return fmt.Sprintf("http://%s", addr)
}

// Pick chooses a random address from a given Peer's addresses, and returns it as
// an addressible URI. If the given peer does not exist, an empty string is returned.
func (ps Cluster) Pick(id int64) string {
	addrs := ps[id]
	if len(addrs) == 0 {
		return ""
	}
	return addScheme(addrs[rand.Intn(len(addrs))])
}

// Set parses command line sets of names to IPs formatted like:
// mach0=1.1.1.1,2.2.2.2 mach0=1.1.1.1 mach1=2.2.2.2,3.3.3.3
func (ps *Cluster) Set(s string) error {
	m := make(map[int64][]string)
	for _, f := range strings.Fields(s) {
		pcnf := strings.SplitN(f, "=", 2)
		if len(pcnf) != 2 {
			return fmt.Errorf("Too few entries for %q", pcnf)
		}
		var nn NodeName
		nn.Set(pcnf[0])
		m[nn.ID()] = strings.Split(pcnf[1], ",")
	}
	*ps = m
	return nil
}

func (ps *Cluster) String() string {
	v := url.Values{}
	for k, vv := range *ps {
		for i := range vv {
			v.Add(strconv.FormatInt(k, 16), vv[i])
		}
	}
	return v.Encode()
}

func (ps Cluster) IDs() []int64 {
	var ids []int64
	for id := range ps {
		ids = append(ids, id)
	}
	return ids
}

type Endpoints []string

func (ep *Endpoints) Set(s string) error {
	endpoints := make([]string, 0)
	for _, addr := range strings.Split(s, ",") {
		endpoints = append(endpoints, addScheme(addr))
	}

	*ep = endpoints

	return nil
}

func (ep *Endpoints) String() string {
	return strings.Join([]string(*ep), ",")
}

func (ep Endpoints) List() []string {
	return []string(ep)
}

// Endpoints returns a list of all peer addresses. Each address is prefixed
// with the scheme (currently "http://"). The returned list is sorted in
// ascending lexicographical order.
func (ps Cluster) Endpoints() Endpoints {
	endpoints := make([]string, 0)
	for _, addrs := range ps {
		for _, addr := range addrs {
			endpoints = append(endpoints, addScheme(addr))
		}
	}
	sort.Strings(endpoints)

	return endpoints
}

// NodeName implements the flag.Value interface for choosing a default
// etcd node name and generating the internal ID using a sha hash.
type NodeName string

func (nn *NodeName) Set(s string) error {
	if s != "" {
		*nn = NodeName(s)
		return nil
	}

	host, err := os.Hostname()
	if host != "" && err == nil {
		*nn = NodeName(host)
		return nil
	}

	*nn = NodeName("default")
	return nil
}

func (nn *NodeName) String() string {
	return string(*nn)
}

func (nn *NodeName) ID() int64 {
	hash := sha1.Sum([]byte(*nn))
	out := int64(binary.BigEndian.Uint64(hash[:8]))
	if out < 0 {
		out = out * -1
	}
	return out
}
