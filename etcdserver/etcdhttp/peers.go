package etcdhttp

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"github.com/coreos/etcd/elog"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/raft/raftpb"
)

// Peers contains a mapping of unique IDs to a list of hostnames/IP addresses
type Peers map[int64][]string

// addScheme adds the protocol prefix to a string; currently only HTTP
// TODO: improve this when implementing TLS
func addScheme(addr string) string {
	return fmt.Sprintf("http://%s", addr)
}

func (ps Peers) Peers() map[int64][]string {
	return ps
}

// Pick chooses a random address from a given Peer's addresses, and returns it as
// an addressible URI. If the given peer does not exist, an empty string is returned.
func (ps Peers) Pick(id int64) string {
	addrs := ps[id]
	if len(addrs) == 0 {
		return ""
	}
	return addScheme(addrs[rand.Intn(len(addrs))])
}

// Set parses command line sets of names to IPs formatted like:
// a=1.1.1.1&a=1.1.1.2&b=2.2.2.2
func (ps *Peers) Set(s string) error {
	m := make(map[int64][]string)
	v, err := url.ParseQuery(s)
	if err != nil {
		return err
	}
	for k, v := range v {
		id, err := strconv.ParseInt(k, 0, 64)
		if err != nil {
			return err
		}
		m[id] = v
	}
	*ps = m
	return nil
}

func (ps *Peers) String() string {
	v := url.Values{}
	for k, vv := range *ps {
		for i := range vv {
			v.Add(strconv.FormatInt(k, 16), vv[i])
		}
	}
	return v.Encode()
}

func (ps Peers) IDs() []int64 {
	var ids []int64
	for id := range ps {
		ids = append(ids, id)
	}
	return ids
}

// Endpoints returns a list of all peer addresses. Each address is prefixed
// with the scheme (currently "http://"). The returned list is sorted in
// ascending lexicographical order.
func (ps Peers) Endpoints() []string {
	endpoints := make([]string, 0)
	for _, addrs := range ps {
		for _, addr := range addrs {
			endpoints = append(endpoints, addScheme(addr))
		}
	}
	sort.Strings(endpoints)

	return endpoints
}

func Sender(pst etcdserver.PeerGetter) func(msgs []raftpb.Message) {
	return func(msgs []raftpb.Message) {
		for _, m := range msgs {
			// TODO: reuse go routines
			// limit the number of outgoing connections for the same receiver
			go send(pst, m)
		}
	}
}

func send(pst etcdserver.PeerGetter, m raftpb.Message) {
	// TODO (xiangli): reasonable retry logic
	for i := 0; i < 3; i++ {
		info := pst.Get(m.To)
		if info.IsEmpty() {
			// TODO: unknown peer id.. what do we do? I
			// don't think his should ever happen, need to
			// look into this further.
			log.Printf("etcdhttp: no addr for %d", m.To)
			return
		}

		url := info.PeerURLs[rand.Intn(len(info.PeerURLs))]
		url += raftPrefix

		// TODO: don't block. we should be able to have 1000s
		// of messages out at a time.
		data, err := m.Marshal()
		if err != nil {
			log.Println("etcdhttp: dropping message:", err)
			return // drop bad message
		}
		if httpPost(url, data) {
			return // success
		}
		// TODO: backoff
	}
}

func httpPost(url string, data []byte) bool {
	// TODO: set timeouts
	resp, err := http.Post(url, "application/protobuf", bytes.NewBuffer(data))
	if err != nil {
		elog.TODO()
		return false
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		elog.TODO()
		return false
	}
	return true
}
