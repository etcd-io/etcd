/*
Copyright 2013 CoreOS Inc.

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

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/go-raft"
)

type raftServer struct {
	*raft.Server
	version        string
	joinIndex      uint64
	name           string
	url            string
	listenHost     string
	tlsConf        *TLSConfig
	tlsInfo        *TLSInfo
	followersStats *raftFollowersStats
	serverStats    *raftServerStats
}

var r *raftServer

func newRaftServer(name string, url string, listenHost string, tlsConf *TLSConfig, tlsInfo *TLSInfo) *raftServer {

	// Create transporter for raft
	raftTransporter := newTransporter(tlsConf.Scheme, tlsConf.Client)

	// Create raft server
	server, err := raft.NewServer(name, dirPath, raftTransporter, etcdStore, nil, "")

	check(err)

	return &raftServer{
		Server:     server,
		version:    raftVersion,
		name:       name,
		url:        url,
		listenHost: listenHost,
		tlsConf:    tlsConf,
		tlsInfo:    tlsInfo,
		followersStats: &raftFollowersStats{
			Leader:    name,
			Followers: make(map[string]*raftFollowerStats),
		},
		serverStats: &raftServerStats{
			StartTime: time.Now(),
			sendRateQueue: &statsQueue{
				back: -1,
			},
			recvRateQueue: &statsQueue{
				back: -1,
			},
		},
	}
}

// Start the raft server
func (r *raftServer) ListenAndServe() {
	// Setup commands.
	registerCommands()

	// LoadSnapshot
	if snapshot {
		err := r.LoadSnapshot()

		if err == nil {
			debugf("%s finished load snapshot", r.name)
		} else {
			debug(err)
		}
	}

	r.SetElectionTimeout(ElectionTimeout)
	r.SetHeartbeatTimeout(HeartbeatTimeout)

	r.Start()

	if r.IsLogEmpty() {

		// start as a leader in a new cluster
		if len(cluster) == 0 {
			startAsLeader()

		} else {
			startAsFollower()
		}

	} else {

		// rejoin the previous cluster
		cluster = getMachines(nameToRaftURL)
		for i := 0; i < len(cluster); i++ {
			u, err := url.Parse(cluster[i])
			if err != nil {
				debug("rejoin cannot parse url: ", err)
			}
			cluster[i] = u.Host
		}
		ok := joinCluster(cluster)
		if !ok {
			warn("the entire cluster is down! this machine will restart the cluster.")
		}

		debugf("%s restart as a follower", r.name)
	}

	// open the snapshot
	if snapshot {
		go monitorSnapshot()
	}

	// start to response to raft requests
	go r.startTransport(r.tlsConf.Scheme, r.tlsConf.Server)

}

func startAsLeader() {
	// leader need to join self as a peer
	for {
		_, err := r.Do(newJoinCommand())
		if err == nil {
			break
		}
	}
	debugf("%s start as a leader", r.name)
}

func startAsFollower() {
	// start as a follower in a existing cluster
	for i := 0; i < retryTimes; i++ {
		ok := joinCluster(cluster)
		if ok {
			return
		}
		warnf("cannot join to cluster via given machines, retry in %d seconds", RetryInterval)
		time.Sleep(time.Second * RetryInterval)
	}

	fatalf("Cannot join the cluster via given machines after %x retries", retryTimes)
}

// Start to listen and response raft command
func (r *raftServer) startTransport(scheme string, tlsConf tls.Config) {
	infof("raft server [name %s, listen on %s, advertised url %s]", r.name, r.listenHost, r.url)

	raftMux := http.NewServeMux()

	server := &http.Server{
		Handler:   raftMux,
		TLSConfig: &tlsConf,
		Addr:      r.listenHost,
	}

	// internal commands
	raftMux.HandleFunc("/name", NameHttpHandler)
	raftMux.HandleFunc("/version", RaftVersionHttpHandler)
	raftMux.Handle("/join", errorHandler(JoinHttpHandler))
	raftMux.HandleFunc("/remove/", RemoveHttpHandler)
	raftMux.HandleFunc("/vote", VoteHttpHandler)
	raftMux.HandleFunc("/log", GetLogHttpHandler)
	raftMux.HandleFunc("/log/append", AppendEntriesHttpHandler)
	raftMux.HandleFunc("/snapshot", SnapshotHttpHandler)
	raftMux.HandleFunc("/snapshotRecovery", SnapshotRecoveryHttpHandler)
	raftMux.HandleFunc("/etcdURL", EtcdURLHttpHandler)

	if scheme == "http" {
		fatal(server.ListenAndServe())
	} else {
		fatal(server.ListenAndServeTLS(r.tlsInfo.CertFile, r.tlsInfo.KeyFile))
	}

}

// getVersion fetches the raft version of a peer. This works for now but we
// will need to do something more sophisticated later when we allow mixed
// version clusters.
func getVersion(t *transporter, versionURL url.URL) (string, error) {
	resp, req, err := t.Get(versionURL.String())

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	t.CancelWhenTimeout(req)

	body, err := ioutil.ReadAll(resp.Body)

	return string(body), nil
}

func joinCluster(cluster []string) bool {
	for _, machine := range cluster {

		if len(machine) == 0 {
			continue
		}

		err := joinByMachine(r.Server, machine, r.tlsConf.Scheme)
		if err == nil {
			debugf("%s success join to the cluster via machine %s", r.name, machine)
			return true

		} else {
			if _, ok := err.(etcdErr.Error); ok {
				fatal(err)
			}

			debugf("cannot join to cluster via machine %s %s", machine, err)
		}
	}
	return false
}

// Send join requests to machine.
func joinByMachine(s *raft.Server, machine string, scheme string) error {
	var b bytes.Buffer

	// t must be ok
	t, _ := r.Transporter().(*transporter)

	// Our version must match the leaders version
	versionURL := url.URL{Host: machine, Scheme: scheme, Path: "/version"}
	version, err := getVersion(t, versionURL)
	if err != nil {
		return fmt.Errorf("Unable to join: %v", err)
	}

	// TODO: versioning of the internal protocol. See:
	// Documentation/internatl-protocol-versioning.md
	if version != r.version {
		return fmt.Errorf("Unable to join: internal version mismatch, entire cluster must be running identical versions of etcd")
	}

	json.NewEncoder(&b).Encode(newJoinCommand())

	joinURL := url.URL{Host: machine, Scheme: scheme, Path: "/join"}

	debugf("Send Join Request to %s", joinURL.String())

	resp, req, err := t.Post(joinURL.String(), &b)

	for {
		if err != nil {
			return fmt.Errorf("Unable to join: %v", err)
		}
		if resp != nil {
			defer resp.Body.Close()

			t.CancelWhenTimeout(req)

			if resp.StatusCode == http.StatusOK {
				b, _ := ioutil.ReadAll(resp.Body)
				r.joinIndex, _ = binary.Uvarint(b)
				return nil
			}
			if resp.StatusCode == http.StatusTemporaryRedirect {

				address := resp.Header.Get("Location")
				debugf("Send Join Request to %s", address)

				json.NewEncoder(&b).Encode(newJoinCommand())

				resp, req, err = t.Post(address, &b)

			} else if resp.StatusCode == http.StatusBadRequest {
				debug("Reach max number machines in the cluster")
				decoder := json.NewDecoder(resp.Body)
				err := &etcdErr.Error{}
				decoder.Decode(err)
				return *err
			} else {
				return fmt.Errorf("Unable to join")
			}
		}

	}
	return fmt.Errorf("Unable to join: %v", err)
}

func (r *raftServer) Stats() []byte {
	r.serverStats.LeaderInfo.Uptime = time.Now().Sub(r.serverStats.LeaderInfo.startTime).String()

	queue := r.serverStats.sendRateQueue

	r.serverStats.SendingPkgRate, r.serverStats.SendingBandwidthRate = queue.Rate()

	queue = r.serverStats.recvRateQueue

	r.serverStats.RecvingPkgRate, r.serverStats.RecvingBandwidthRate = queue.Rate()

	b, _ := json.Marshal(r.serverStats)

	return b
}

func (r *raftServer) PeerStats() []byte {
	if r.State() == raft.Leader {
		b, _ := json.Marshal(r.followersStats)
		return b
	}
	return nil
}

// Register commands to raft server
func registerCommands() {
	raft.RegisterCommand(&JoinCommand{})
	raft.RegisterCommand(&RemoveCommand{})
	raft.RegisterCommand(&SetCommand{})
	raft.RegisterCommand(&GetCommand{})
	raft.RegisterCommand(&DeleteCommand{})
	raft.RegisterCommand(&WatchCommand{})
	raft.RegisterCommand(&TestAndSetCommand{})
}
