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
	"os"
	"time"

	"github.com/coreos/etcd/proxy/tcpproxy"
	"github.com/spf13/cobra"
)

var (
	gatewayListenAddr string
	gatewayEndpoints  []string
	getewayRetryDelay time.Duration
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
	cmd.Flags().StringSliceVar(&gatewayEndpoints, "endpoints", []string{"127.0.0.1:2379"}, "comma separated etcd cluster endpoints")
	cmd.Flags().DurationVar(&getewayRetryDelay, "retry-delay", time.Minute, "duration of delay before retrying failed endpoints")

	return &cmd
}

func startGateway(cmd *cobra.Command, args []string) {
	l, err := net.Listen("tcp", gatewayListenAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	tp := tcpproxy.TCPProxy{
		Listener:        l,
		Endpoints:       gatewayEndpoints,
		MonitorInterval: getewayRetryDelay,
	}

	tp.Run()
}
