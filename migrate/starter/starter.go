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

package starter

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/etcdmain"
	"github.com/coreos/etcd/migrate"
	"github.com/coreos/etcd/pkg/flags"
	"github.com/coreos/etcd/wal"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

type version string

const (
	internalV1      version = "1"
	internalV2      version = "2"
	internalUnknown version = "unknown"

	defaultInternalV1etcdBinaryDir = "/usr/libexec/etcd/versions/"
)

func StartDesiredVersion(args []string) {
	switch checkStartVersion(args) {
	case internalV1:
		startInternalV1()
	case internalV2:
	default:
		log.Panicf("migrate: unhandled start version")
	}
}

func checkStartVersion(args []string) version {
	fs, err := parseConfig(args)
	if err != nil {
		return internalV2
	}
	// If it uses 2.0 env var explicitly, start 2.0
	if fs.Lookup("initial-cluster").Value.String() != "" {
		return internalV2
	}

	dataDir := fs.Lookup("data-dir").Value.String()
	if dataDir == "" {
		log.Fatalf("migrate: please set ETCD_DATA_DIR for etcd")
	}
	// check the data directory
	walVersion, err := wal.DetectVersion(dataDir)
	if err != nil {
		log.Fatalf("migrate: failed to detect etcd version in %v: %v", dataDir, err)
	}
	log.Printf("migrate: detect etcd version %s in %s", walVersion, dataDir)
	switch walVersion {
	case wal.WALv0_5:
		return internalV2
	case wal.WALv0_4:
		// TODO: standby case
		// if it is standby guy:
		//     print out detect standby mode
		//     go to WALNotExist case
		//     if want to start with 2.0:
		//         remove old data dir to avoid auto migration
		//         try to let it fallback? or use local proxy file?
		ver, err := checkStartVersionByDataDir4(dataDir)
		if err != nil {
			log.Fatalf("migrate: failed to check start version in %v: %v", dataDir, err)
		}
		return ver
	case wal.WALUnknown:
		log.Fatalf("migrate: unknown etcd version in %v", dataDir)
	case wal.WALNotExist:
		discovery := fs.Lookup("discovery").Value.String()
		peers := trimSplit(fs.Lookup("peers").Value.String(), ",")
		peerTLSInfo := &TLSInfo{
			CAFile:   fs.Lookup("peer-ca-file").Value.String(),
			CertFile: fs.Lookup("peer-cert-file").Value.String(),
			KeyFile:  fs.Lookup("peer-key-file").Value.String(),
		}
		ver, err := checkStartVersionByMembers(discovery, peers, peerTLSInfo)
		if err != nil {
			log.Printf("migrate: failed to check start version through peers: %v", err)
			break
		}
		return ver
	default:
		log.Panicf("migrate: unhandled etcd version in %v", dataDir)
	}
	return internalV2
}

func checkStartVersionByDataDir4(dataDir string) (version, error) {
	// check v0.4 snapshot
	snap4, err := migrate.DecodeLatestSnapshot4FromDir(snapDir4(dataDir))
	if err != nil {
		return internalUnknown, err
	}
	if snap4 != nil {
		st := &migrate.Store4{}
		if err := json.Unmarshal(snap4.State, st); err != nil {
			return internalUnknown, err
		}
		dir := st.Root.Children["_etcd"]
		n, ok := dir.Children["next-internal-version"]
		if ok && n.Value == "2" {
			return internalV2, nil
		}
	}

	// check v0.4 log
	ents4, err := migrate.DecodeLog4FromFile(logFile4(dataDir))
	if err != nil {
		return internalUnknown, err
	}
	for _, e := range ents4 {
		cmd, err := migrate.NewCommand4(e.GetCommandName(), e.GetCommand(), nil)
		if err != nil {
			return internalUnknown, err
		}
		setcmd, ok := cmd.(*migrate.SetCommand)
		if !ok {
			continue
		}
		if setcmd.Key == "/_etcd/next-internal-version" && setcmd.Value == "2" {
			return internalV2, nil
		}
	}
	return internalV1, nil
}

func checkStartVersionByMembers(discoverURL string, peers []string, tls *TLSInfo) (version, error) {
	tr := &http.Transport{}
	if tls.Scheme() == "https" {
		tlsConfig, err := tls.ClientConfig()
		if err != nil {
			return internalUnknown, err
		}
		tr.TLSClientConfig = tlsConfig
	}
	c := &http.Client{Transport: tr}

	possiblePeers, err := getPeersFromDiscoveryURL(discoverURL)
	if err != nil {
		return internalUnknown, err
	}
	for _, p := range peers {
		possiblePeers = append(possiblePeers, tls.Scheme()+"://"+p)
	}

	for _, p := range possiblePeers {
		resp, err := c.Get(p + "/etcdURL")
		if err != nil {
			log.Printf("migrate: failed to get /etcdURL from %s", p)
			continue
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("migrate: failed to read body from %s", p)
			continue
		}
		resp, err = c.Get(string(b) + "/version")
		if err != nil {
			log.Printf("migrate: failed to get /version from %s", p)
			continue
		}
		b, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("migrate: failed to read body from %s", p)
			continue
		}

		var m map[string]string
		err = json.Unmarshal(b, &m)
		if err != nil {
			log.Printf("migrate: failed to unmarshal body %s from %s", b, p)
			continue
		}
		switch m["internalVersion"] {
		case "1":
			return internalV1, nil
		case "2":
			return internalV2, nil
		default:
			log.Printf("migrate: unrecognized internal version %s from %s", m["internalVersion"], p)
		}
	}
	return internalUnknown, fmt.Errorf("failed to get version from peers %v", possiblePeers)
}

func getPeersFromDiscoveryURL(discoverURL string) ([]string, error) {
	if discoverURL == "" {
		return nil, nil
	}

	u, err := url.Parse(discoverURL)
	if err != nil {
		return nil, err
	}
	token := u.Path
	u.Path = ""
	c, err := client.NewHTTPClient(&http.Transport{}, []string{u.String()})
	if err != nil {
		return nil, err
	}
	dc := client.NewDiscoveryKeysAPI(c)

	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	resp, err := dc.Get(ctx, token)
	cancel()
	if err != nil {
		return nil, err
	}
	peers := make([]string, 0)
	// append non-config keys to peers
	for _, n := range resp.Node.Nodes {
		if g := path.Base(n.Key); g == "_config" || g == "_state" {
			continue
		}
		peers = append(peers, n.Value)
	}
	return peers, nil
}

func startInternalV1() {
	p := os.Getenv("ETCD_BINARY_DIR")
	if p == "" {
		p = defaultInternalV1etcdBinaryDir
	}
	p = path.Join(p, "1")
	err := syscall.Exec(p, os.Args, syscall.Environ())
	if err != nil {
		log.Fatalf("migrate: failed to execute internal v1 etcd: %v", err)
	}
}

type value struct {
	s string
}

func (v *value) String() string { return v.s }

func (v *value) Set(s string) error {
	v.s = s
	return nil
}

// parseConfig parses out the input config from cmdline arguments and
// environment variables.
func parseConfig(args []string) (*flag.FlagSet, error) {
	fs := flag.NewFlagSet("full flagset", flag.ContinueOnError)
	etcdmain.NewConfig().VisitAll(func(f *flag.Flag) {
		fs.Var(&value{}, f.Name, "")
	})
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if err := flags.SetFlagsFromEnv(fs); err != nil {
		return nil, err
	}
	return fs, nil
}

func snapDir4(dataDir string) string {
	return path.Join(dataDir, "snapshot")
}

func logFile4(dataDir string) string {
	return path.Join(dataDir, "log")
}

func trimSplit(s, sep string) []string {
	trimmed := strings.Split(s, sep)
	for i := range trimmed {
		trimmed[i] = strings.TrimSpace(trimmed[i])
	}
	return trimmed
}
