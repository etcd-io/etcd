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

package command

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/bgentry/speakeasy"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/transport"
)

var (
	ErrNoAvailSrc = errors.New("no available argument and stdin")
)

// trimsplit slices s into all substrings separated by sep and returns a
// slice of the substrings between the separator with all leading and trailing
// white space removed, as defined by Unicode.
func trimsplit(s, sep string) []string {
	raw := strings.Split(s, ",")
	trimmed := make([]string, 0)
	for _, r := range raw {
		trimmed = append(trimmed, strings.TrimSpace(r))
	}
	return trimmed
}

func argOrStdin(args []string, stdin io.Reader, i int) (string, error) {
	if i < len(args) {
		return args[i], nil
	}
	bytes, err := ioutil.ReadAll(stdin)
	if string(bytes) == "" || err != nil {
		return "", ErrNoAvailSrc
	}
	return string(bytes), nil
}

func getPeersFlagValue(c *cli.Context) []string {
	peerstr := c.GlobalString("peers")

	// Use an environment variable if nothing was supplied on the
	// command line
	if peerstr == "" {
		peerstr = os.Getenv("ETCDCTL_PEERS")
	}

	// If we still don't have peers, use a default
	if peerstr == "" {
		peerstr = "http://127.0.0.1:4001,http://127.0.0.1:2379"
	}

	return strings.Split(peerstr, ",")
}

func getDomainDiscoveryFlagValue(c *cli.Context) ([]string, error) {
	domainstr := c.GlobalString("discovery-srv")

	// Use an environment variable if nothing was supplied on the
	// command line
	if domainstr == "" {
		domainstr = os.Getenv("ETCDCTL_DISCOVERY_SRV")
	}

	// If we still don't have domain discovery, return nothing
	if domainstr == "" {
		return []string{}, nil
	}

	discoverer := client.NewSRVDiscover()
	eps, err := discoverer.Discover(domainstr)
	if err != nil {
		return nil, err
	}

	return eps, err
}

func getEndpoints(c *cli.Context) ([]string, error) {
	eps, err := getDomainDiscoveryFlagValue(c)
	if err != nil {
		return nil, err
	}

	// If domain discovery returns no endpoints, check peer flag
	if len(eps) == 0 {
		eps = getPeersFlagValue(c)
	}

	for i, ep := range eps {
		u, err := url.Parse(ep)
		if err != nil {
			return nil, err
		}

		if u.Scheme == "" {
			u.Scheme = "http"
		}

		eps[i] = u.String()
	}

	return eps, nil
}

func getTransport(c *cli.Context) (*http.Transport, error) {
	cafile := c.GlobalString("ca-file")
	certfile := c.GlobalString("cert-file")
	keyfile := c.GlobalString("key-file")

	// Use an environment variable if nothing was supplied on the
	// command line
	if cafile == "" {
		cafile = os.Getenv("ETCDCTL_CA_FILE")
	}
	if certfile == "" {
		certfile = os.Getenv("ETCDCTL_CERT_FILE")
	}
	if keyfile == "" {
		keyfile = os.Getenv("ETCDCTL_KEY_FILE")
	}

	tls := transport.TLSInfo{
		CAFile:   cafile,
		CertFile: certfile,
		KeyFile:  keyfile,
	}
	return transport.NewTransport(tls)
}

func getUsernamePasswordFromFlag(usernameFlag string) (username string, password string, err error) {
	colon := strings.Index(usernameFlag, ":")
	if colon == -1 {
		username = usernameFlag
		// Prompt for the password.
		password, err = speakeasy.Ask("Password: ")
		if err != nil {
			return "", "", err
		}
	} else {
		username = usernameFlag[:colon]
		password = usernameFlag[colon+1:]
	}
	return username, password, nil
}

func mustNewKeyAPI(c *cli.Context) client.KeysAPI {
	return client.NewKeysAPI(mustNewClient(c))
}

func mustNewMembersAPI(c *cli.Context) client.MembersAPI {
	return client.NewMembersAPI(mustNewClient(c))
}

func mustNewClient(c *cli.Context) client.Client {
	eps, err := getEndpoints(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	tr, err := getTransport(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	cfg := client.Config{
		Transport: tr,
		Endpoints: eps,
	}

	uFlag := c.GlobalString("username")
	if uFlag != "" {
		username, password, err := getUsernamePasswordFromFlag(uFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		cfg.Username = username
		cfg.Password = password
	}

	hc, err := client.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	if !c.GlobalBool("no-sync") {
		ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
		err := hc.Sync(ctx)
		cancel()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}

	if c.GlobalBool("debug") {
		fmt.Fprintf(os.Stderr, "Cluster-Endpoints: %s\n", strings.Join(hc.Endpoints(), ", "))
		client.EnablecURLDebug()
	}

	return hc
}
