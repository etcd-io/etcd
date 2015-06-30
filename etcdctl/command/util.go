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

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
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

func getPeersFlagValue(cmd *cobra.Command) []string {
	peerstr, _ := cmd.Flags().GetString("peers")

	// Use an environment variable if nothing was supplied on the
	// command line
	if peerstr == "" {
		peerstr = os.Getenv("ETCDCTL_PEERS")
	}

	// If we still don't have peers, use a default
	if peerstr == "" {
		peerstr = "127.0.0.1:4001,127.0.0.1:2379"
	}

	return strings.Split(peerstr, ",")
}

func getEndpoints(cmd *cobra.Command) ([]string, error) {
	eps := getPeersFlagValue(cmd)
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

func getTransport(cmd *cobra.Command) (*http.Transport, error) {
	cafile, _ := cmd.Flags().GetString("ca-file")
	certfile, _ := cmd.Flags().GetString("cert-file")
	keyfile, _ := cmd.Flags().GetString("key-file")

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

func mustNewClient(cmd *cobra.Command) client.Client {
	eps, err := getEndpoints(cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	tr, err := getTransport(cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	cfg := client.Config{
		Transport: tr,
		Endpoints: eps,
	}

	uFlag, _ := cmd.Flags().GetString("username")
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
	return hc
}
