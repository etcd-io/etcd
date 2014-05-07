package discovery

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
)

const (
	stateKey     = "_state"
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

func (d *Discoverer) Do(discoveryURL string, name string, peer string, closeChan <-chan bool, startRoutine func(func())) (peers []string, err error) {
	d.name = name
	d.peer = peer
	d.discoveryURL = discoveryURL

	u, err := url.Parse(discoveryURL)

	if err != nil {
		return
	}

	// prefix is prepended to all keys for this discovery
	d.prefix = strings.TrimPrefix(u.Path, "/v2/keys/")

	// keep the old path in case we need to set the KeyPrefix below
	oldPath := u.Path
	u.Path = ""

	// Connect to a scheme://host not a full URL with path
	log.Infof("Discovery via %s using prefix %s.", u.String(), d.prefix)
	d.client = etcd.NewClient([]string{u.String()})

	if !strings.HasPrefix(oldPath, "/v2/keys") {
		d.client.SetKeyPrefix("")
	}

	// Register this machine first and announce that we are a member of
	// this cluster
	err = d.heartbeat()
	if err != nil {
		return
	}

	// Start the very slow heartbeat to the cluster now in anticipation
	// that everything is going to go alright now
	startRoutine(func() { d.startHeartbeat(closeChan) })

	// Attempt to take the leadership role, if there is no error we are it!
	resp, err := d.client.Create(path.Join(d.prefix, stateKey), startedState, 0)

	// Bail out on unexpected errors
	if err != nil {
		if clientErr, ok := err.(*etcd.EtcdError); !ok || clientErr.ErrorCode != etcdErr.EcodeNodeExist {
			return nil, err
		}
	}

	// If we got a response then the CAS was successful, we are leader
	if resp != nil && resp.Node.Value == startedState {
		// We are the leader, we have no peers
		log.Infof("Discovery _state was empty, so this machine is the initial leader.")
		return nil, nil
	}

	// Fall through to finding the other discovery peers
	return d.findPeers()
}

func (d *Discoverer) findPeers() (peers []string, err error) {
	resp, err := d.client.Get(path.Join(d.prefix), false, true)
	if err != nil {
		return nil, err
	}

	node := resp.Node

	if node == nil {
		return nil, fmt.Errorf("%s key doesn't exist.", d.prefix)
	}

	for _, n := range node.Nodes {
		// Skip our own entry in the list, there is no point
		if strings.HasSuffix(n.Key, "/"+d.name) {
			continue
		}
		peers = append(peers, n.Value)
	}

	if len(peers) == 0 {
		return nil, errors.New("Discovery found an initialized cluster but no reachable peers are registered.")
	}

	log.Infof("Discovery found peers %v", peers)

	return
}

func (d *Discoverer) startHeartbeat(closeChan <-chan bool) {
	// In case of errors we should attempt to heartbeat fairly frequently
	heartbeatInterval := defaultTTL / 8
	ticker := time.NewTicker(time.Second * time.Duration(heartbeatInterval))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := d.heartbeat()
			if err != nil {
				log.Warnf("Discovery heartbeat failed: %v", err)
			}
		case <-closeChan:
			return
		}
	}
}

func (d *Discoverer) heartbeat() error {
	_, err := d.client.Set(path.Join(d.prefix, d.name), d.peer, defaultTTL)
	return err
}

func Do(discoveryURL string, name string, peer string, closeChan <-chan bool, startRoutine func(func())) ([]string, error) {
	return defaultDiscoverer.Do(discoveryURL, name, peer, closeChan, startRoutine)
}
