// Copyright 2016 The etcd Authors
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

package etcdmain

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/logutil"
	"go.etcd.io/etcd/server/v3/proxy/tcpproxy"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	gatewayListenAddr            string
	gatewayEndpoints             []string
	gatewayDNSCluster            string
	gatewayDNSClusterServiceName string
	gatewayInsecureDiscovery     bool
	gatewayRetryDelay            time.Duration
	gatewayCA                    string
)

var (
	rootCmd = &cobra.Command{
		Use:        "etcd",
		Short:      "etcd server",
		SuggestFor: []string{"etcd"},
	}
)

func init() {
	rootCmd.AddCommand(newGatewayCommand())
}

// newGatewayCommand returns the cobra command for "gateway".
func newGatewayCommand() *cobra.Command {
	lpc := &cobra.Command{
		Use:   "gateway <subcommand>",
		Short: "gateway related command",
	}
	lpc.AddCommand(newGatewayStartCommand())

	return lpc
}

func newGatewayStartCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "start",
		Short: "start the gateway",
		Run:   startGateway,
	}

	cmd.Flags().StringVar(&gatewayListenAddr, "listen-addr", "127.0.0.1:23790", "listen address")
	cmd.Flags().StringVar(&gatewayDNSCluster, "discovery-srv", "", "DNS domain used to bootstrap initial cluster")
	cmd.Flags().StringVar(&gatewayDNSClusterServiceName, "discovery-srv-name", "", "service name to query when using DNS discovery")
	cmd.Flags().BoolVar(&gatewayInsecureDiscovery, "insecure-discovery", false, "accept insecure SRV records")
	cmd.Flags().StringVar(&gatewayCA, "trusted-ca-file", "", "path to the client server TLS CA file for verifying the discovered endpoints when discovery-srv is provided.")

	cmd.Flags().StringSliceVar(&gatewayEndpoints, "endpoints", []string{"127.0.0.1:2379"}, "comma separated etcd cluster endpoints")

	cmd.Flags().DurationVar(&gatewayRetryDelay, "retry-delay", time.Minute, "duration of delay before retrying failed endpoints")

	return &cmd
}

func stripSchema(eps []string) []string {
	var endpoints []string
	for _, ep := range eps {
		if u, err := url.Parse(ep); err == nil && u.Host != "" {
			ep = u.Host
		}
		endpoints = append(endpoints, ep)
	}
	return endpoints
}

func startGateway(cmd *cobra.Command, args []string) {
	lg, err := logutil.CreateDefaultZapLogger(zap.InfoLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// We use os.Args to show all the arguments (not only passed-through Cobra).
	lg.Info("Running: ", zap.Strings("args", os.Args))

	srvs := discoverEndpoints(lg, gatewayDNSCluster, gatewayCA, gatewayInsecureDiscovery, gatewayDNSClusterServiceName)
	if len(srvs.Endpoints) == 0 {
		// no endpoints discovered, fall back to provided endpoints
		srvs.Endpoints = gatewayEndpoints
	}
	// Strip the schema from the endpoints because we start just a TCP proxy
	srvs.Endpoints = stripSchema(srvs.Endpoints)
	if len(srvs.SRVs) == 0 {
		for _, ep := range srvs.Endpoints {
			h, p, serr := net.SplitHostPort(ep)
			if serr != nil {
				fmt.Printf("error parsing endpoint %q", ep)
				os.Exit(1)
			}
			var port uint16
			fmt.Sscanf(p, "%d", &port)
			srvs.SRVs = append(srvs.SRVs, &net.SRV{Target: h, Port: port})
		}
	}

	lhost, lport, err := net.SplitHostPort(gatewayListenAddr)
	if err != nil {
		fmt.Println("failed to validate listen address:", gatewayListenAddr)
		os.Exit(1)
	}

	laddrs, err := net.LookupHost(lhost)
	if err != nil {
		fmt.Println("failed to resolve listen host:", lhost)
		os.Exit(1)
	}
	laddrsMap := make(map[string]bool)
	for _, addr := range laddrs {
		laddrsMap[addr] = true
	}

	for _, srv := range srvs.SRVs {
		var eaddrs []string
		eaddrs, err = net.LookupHost(srv.Target)
		if err != nil {
			fmt.Println("failed to resolve endpoint host:", srv.Target)
			os.Exit(1)
		}
		if fmt.Sprintf("%d", srv.Port) != lport {
			continue
		}

		for _, ea := range eaddrs {
			if laddrsMap[ea] {
				fmt.Printf("SRV or endpoint (%s:%d->%s:%d) should not resolve to the gateway listen addr (%s)\n", srv.Target, srv.Port, ea, srv.Port, gatewayListenAddr)
				os.Exit(1)
			}
		}
	}

	if len(srvs.Endpoints) == 0 {
		fmt.Println("no endpoints found")
		os.Exit(1)
	}

	var l net.Listener
	l, err = net.Listen("tcp", gatewayListenAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	tp := tcpproxy.TCPProxy{
		Logger:          lg,
		Listener:        l,
		Endpoints:       srvs.SRVs,
		MonitorInterval: gatewayRetryDelay,
	}

	// At this point, etcd gateway listener is initialized
	notifySystemd(lg)

	tp.Run()
}
