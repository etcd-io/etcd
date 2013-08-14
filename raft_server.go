package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-raft"
)

type raftServer struct {
	name    string
	url     string
	tlsConf *TLSConfig
	tlsInfo *TLSInfo
	server  *raft.Server
}

var r *raftServer

func newRaftServer(name string, url string, tlsConf *TLSConfig, tlsInfo *TLSInfo) *raftServer {
	return &raftServer{
		name:    name,
		url:     url,
		tlsConf: tlsConf,
		tlsInfo: tlsInfo,
	}
}

// Start the raft server
func (r *raftServer) start() {

	// Setup commands.
	registerCommands()

	// Create transporter for raft
	raftTransporter := newTransporter(r.tlsConf.Scheme, r.tlsConf.Client)

	// Create raft server
	server, err := raft.NewServer(r.name, dirPath, raftTransporter, etcdStore, nil)

	if err != nil {
		fatal(err)
	}

	r.server = server

	// LoadSnapshot
	if snapshot {
		err = server.LoadSnapshot()

		if err == nil {
			debugf("%s finished load snapshot", r.name)
		} else {
			debug(err)
		}
	}

	server.SetElectionTimeout(ElectionTimeout)
	server.SetHeartbeatTimeout(HeartbeatTimeout)

	server.Start()

	if server.IsLogEmpty() {

		// start as a leader in a new cluster
		if len(cluster) == 0 {

			time.Sleep(time.Millisecond * 20)

			// leader need to join self as a peer
			for {
				_, err := server.Do(newJoinCommand())
				if err == nil {
					break
				}
			}
			debugf("%s start as a leader", r.name)

			// start as a follower in a existing cluster
		} else {

			time.Sleep(time.Millisecond * 20)

			for i := 0; i < retryTimes; i++ {

				success := false
				for _, machine := range cluster {
					if len(machine) == 0 {
						continue
					}
					err = joinCluster(server, machine, r.tlsConf.Scheme)
					if err != nil {
						if err.Error() == errors[103] {
							fatal(err)
						}
						debugf("cannot join to cluster via machine %s %s", machine, err)
					} else {
						success = true
						break
					}
				}

				if success {
					break
				}

				warnf("cannot join to cluster via given machines, retry in %d seconds", RetryInterval)
				time.Sleep(time.Second * RetryInterval)
			}
			if err != nil {
				fatalf("Cannot join the cluster via given machines after %x retries", retryTimes)
			}
			debugf("%s success join to the cluster", r.name)
		}

	} else {
		// rejoin the previous cluster
		debugf("%s restart as a follower", r.name)
	}

	// open the snapshot
	if snapshot {
		go monitorSnapshot()
	}

	// start to response to raft requests
	go r.startTransport(r.tlsConf.Scheme, r.tlsConf.Server)

}

// Start to listen and response raft command
func (r *raftServer) startTransport(scheme string, tlsConf tls.Config) {
	u, _ := url.Parse(r.url)
	infof("raft server [%s:%s]", r.name, u)

	raftMux := http.NewServeMux()

	server := &http.Server{
		Handler:   raftMux,
		TLSConfig: &tlsConf,
		Addr:      u.Host,
	}

	// internal commands
	raftMux.HandleFunc("/name", NameHttpHandler)
	raftMux.HandleFunc("/join", JoinHttpHandler)
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

// Send join requests to the leader.
func joinCluster(s *raft.Server, raftURL string, scheme string) error {
	var b bytes.Buffer

	json.NewEncoder(&b).Encode(newJoinCommand())

	// t must be ok
	t, ok := r.server.Transporter().(transporter)

	if !ok {
		panic("wrong type")
	}

	joinURL := url.URL{Host: raftURL, Scheme: scheme, Path: "/join"}

	debugf("Send Join Request to %s", raftURL)

	resp, err := t.Post(joinURL.String(), &b)

	for {
		if err != nil {
			return fmt.Errorf("Unable to join: %v", err)
		}
		if resp != nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			if resp.StatusCode == http.StatusTemporaryRedirect {

				address := resp.Header.Get("Location")
				debugf("Send Join Request to %s", address)

				json.NewEncoder(&b).Encode(newJoinCommand())

				resp, err = t.Post(address, &b)

			} else if resp.StatusCode == http.StatusBadRequest {
				debug("Reach max number machines in the cluster")
				return fmt.Errorf(errors[103])
			} else {
				return fmt.Errorf("Unable to join")
			}
		}

	}
	return fmt.Errorf("Unable to join: %v", err)
}

// Register commands to raft server
func registerCommands() {
	raft.RegisterCommand(&JoinCommand{})
	raft.RegisterCommand(&SetCommand{})
	raft.RegisterCommand(&GetCommand{})
	raft.RegisterCommand(&DeleteCommand{})
	raft.RegisterCommand(&WatchCommand{})
	raft.RegisterCommand(&TestAndSetCommand{})
}
