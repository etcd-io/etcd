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

var raftTransporter transporter
var raftServer *raft.Server

// Start the raft server
func startRaft(tlsConfig TLSConfig) {
	if veryVerbose {
		raft.SetLogLevel(raft.Debug)
	}

	var err error

	raftName := info.Name

	// Create transporter for raft
	raftTransporter = newTransporter(tlsConfig.Scheme, tlsConfig.Client)

	// Create raft server
	raftServer, err = raft.NewServer(raftName, dirPath, raftTransporter, etcdStore, nil)

	if err != nil {
		fatal(err)
	}

	// LoadSnapshot
	if snapshot {
		err = raftServer.LoadSnapshot()

		if err == nil {
			debugf("%s finished load snapshot", raftServer.Name())
		} else {
			debug(err)
		}
	}

	raftServer.SetElectionTimeout(ElectionTimeout)
	raftServer.SetHeartbeatTimeout(HeartbeatTimeout)

	raftServer.Start()

	if raftServer.IsLogEmpty() {
		// TODO(XiangLi): What is this sleep for? Please document.
		time.Sleep(time.Millisecond * 20)
		if len(cluster) == 0 {
			startAsLeader()
		} else {
			startAsFollower(tlsConfig)
		}
	} else {
		// rejoin the previous cluster
		debugf("%s restart as a follower", raftServer.Name())
	}

	// open the snapshot
	if snapshot {
		go monitorSnapshot()
	}

	// start to response to raft requests
	go startRaftTransport(*info, tlsConfig.Scheme, tlsConfig.Server)

}

// startAsLeader starts this machines as a leader in a new cluster.
func startAsLeader() {
	// leader needs to join to itself as a peer
	for {
		command := &JoinCommand{
			Name:    raftServer.Name(),
			RaftURL: argInfo.RaftURL,
			EtcdURL: argInfo.EtcdURL,
		}
		_, err := raftServer.Do(command)
		if err == nil {
			break
		}
	}

	debugf("%s start as a leader", raftServer.Name())
	return
}

// startAsFollower starts this machine as a follower in a existing cluster.
func startAsFollower(tlsConfig TLSConfig) {
	var err error
	for i := 0; i < retryTimes; i++ {
		success := false
		for _, machine := range cluster {
			if len(machine) == 0 {
				continue
			}

			err = joinCluster(raftServer, machine, tlsConfig.Scheme)

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

	debugf("%s success join to the cluster", raftServer.Name())

	return
}

// Start to listen and response raft command
func startRaftTransport(info Info, scheme string, tlsConf tls.Config) {
	u, _ := url.Parse(info.RaftURL)
	infof("raft server [%s:%s]", info.Name, u)

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
		fatal(server.ListenAndServeTLS(info.RaftTLS.CertFile, info.RaftTLS.KeyFile))
	}

}

// Send join requests to the leader.
func joinCluster(s *raft.Server, raftURL string, scheme string) error {
	var b bytes.Buffer

	command := &JoinCommand{
		Name:    s.Name(),
		RaftURL: info.RaftURL,
		EtcdURL: info.EtcdURL,
	}

	json.NewEncoder(&b).Encode(command)

	// t must be ok
	t, ok := raftServer.Transporter().(transporter)

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

				json.NewEncoder(&b).Encode(command)

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
