// Copyright 2025 The etcd Authors
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
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/engine"
	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/engine/intf"
	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/plugins/epstatus"
	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/plugins/membership"
	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/plugins/metrics"
	readplugin "go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/plugins/read"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
)

var (
	useCluster   bool
	dbQuotaBytes int64
	outputFile   string
)

// NewDiagnosisCommand returns the cobra command for "diagnosis".
func NewDiagnosisCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "diagnosis",
		Short:   "One-stop etcd diagnosis tool",
		Run:     runDiagnosis,
		GroupID: groupClusterMaintenanceID,
	}

	cmd.Flags().BoolVar(&useCluster, "cluster", false, "use all endpoints from the cluster member list")
	cmd.Flags().Int64Var(&dbQuotaBytes, "etcd-storage-quota-bytes", 2*1024*1024*1024, "etcd storage quota in bytes (the value passed to etcd instance by flag --quota-backend-bytes)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "write report to file instead of stdout")

	return cmd
}

func runDiagnosis(cmd *cobra.Command, args []string) {
	cfg := clientConfigFromCmd(cmd)
	cli := mustClientFromCmd(cmd)
	defer cli.Close()

	eps := cfg.Endpoints
	if useCluster {
		ctx, cancel := commandCtx(cmd)
		members, err := cli.MemberList(ctx)
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to fetch member list: %v\n", err)
			os.Exit(cobrautl.ExitError)
		}
		var clusterEps []string
		for _, m := range members.Members {
			clusterEps = append(clusterEps, m.ClientURLs...)
		}
		eps = clusterEps
		cfg.Endpoints = eps
	}

	timeout, err := cmd.Flags().GetDuration("command-timeout")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get command-timeout: %v\n", err)
		os.Exit(cobrautl.ExitError)
	}

	plugins := []intf.Plugin{
		membership.NewPlugin(cfg, eps, timeout),
		epstatus.NewPlugin(cfg, eps, timeout, dbQuotaBytes),
		readplugin.NewPlugin(cfg, eps, timeout, false),
		readplugin.NewPlugin(cfg, eps, timeout, true),
		metrics.NewPlugin(cfg, eps, timeout),
	}

	report, err := engine.Diagnose(cfg, plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "diagnosis failed: %v\n", err)
		os.Exit(cobrautl.ExitError)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, report, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write report: %v\n", err)
			os.Exit(cobrautl.ExitError)
		}
		return
	}

	fmt.Fprintln(os.Stdout, string(report))
}
