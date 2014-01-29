package discovery

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/coreos/etcd/log"
	"github.com/coreos/go-etcd/etcd"
)

const (
	stateKey     = "_state"
	initState    = "init"
	startedState = "started"
	defaultTTL   = 604800 // One week TTL
)

type Discoverer struct {
	client       *etcd.Client
	name         string
	peer         string
	prefix       string
	discoveryURL string
}

var defaultDiscoverer *Discoverer

func init() {
	defaultDiscoverer = &Discoverer{}
}

func (d *Discoverer) Do(discoveryURL string, name string, peer string) (peers []string, err error) {
	d.name = name
	d.peer = peer
	d.discoveryURL = discoveryURL

	u, err := url.Parse(discoveryURL)

	if err != nil {
		return
	}

	// prefix is appended to all keys
	d.prefix = strings.TrimPrefix(u.Path, "/v2/keys/")

	// Connect to a scheme://host not a full URL with path
	u.Path = ""
	log.Infof("Bootstrapping via %s using prefix %s.", u.String(), d.prefix)
	d.client = etcd.NewClient([]string{u.String()})

	// Register this machine first and announce that we are a member of
	// this cluster
	err = d.heartbeat()
	if err != nil {
		return
	}

	// Start the very slow heartbeat to the cluster now in anticipation
	// that everything is going to go alright now
	go d.startHeartbeat()

	// Attempt to take the leadership role, if there is no error we are it!
	resp, err := d.client.CompareAndSwap(path.Join(d.prefix, stateKey), startedState, 0, initState, 0)

	// Bail out on unexpected errors
	if err != nil {
		if etcdErr, ok := err.(etcd.EtcdError); !ok || etcdErr.ErrorCode != 101 {
			return nil, err
		}
	}

	// If we got a response then the CAS was successful, we are leader
	if resp != nil && resp.Node.Value == startedState {
		// We are the leader, we have no peers
		log.Infof("Bootstrapping was in 'init' state this machine is the initial leader.")
		return nil, nil
	}

	// Fall through to finding the other discoveryped peers
	return d.findPeers()
}

func (d *Discoverer) findPeers() (peers []string, err error) {
	resp, err := d.client.Get(path.Join(d.prefix), false, true)
	if err != nil {
		return nil, err
	}

	node := resp.Node

	if node == nil {
		return nil, errors.New(fmt.Sprintf("%s key doesn't exist.", d.prefix))
	}

	for _, n := range node.Nodes {
		// Skip our own entry in the list, there is no point
		if strings.HasSuffix(n.Key, "/"+d.name) {
			continue
		}
		peers = append(peers, n.Value)
	}

	if len(peers) == 0 {
		return nil, errors.New("No peers found.")
	}

	log.Infof("Bootstrap found peers %v", peers)

	return
}

func (d *Discoverer) startHeartbeat() {
	// In case of errors we should attempt to heartbeat fairly frequently
	heartbeatInterval := defaultTTL / 8
	ticker := time.Tick(time.Second * time.Duration(heartbeatInterval))
	for {
		select {
		case <-ticker:
			err := d.heartbeat()
			if err != nil {
				log.Warnf("Bootstrapping heartbeat failed: %v", err)
			}
		}
	}
}

func (d *Discoverer) heartbeat() error {
	_, err := d.client.Set(path.Join(d.prefix, d.name), d.peer, defaultTTL)

	return err
}

func Do(discoveryURL string, name string, peer string) ([]string, error) {
	return defaultDiscoverer.Do(discoveryURL, name, peer)
}
