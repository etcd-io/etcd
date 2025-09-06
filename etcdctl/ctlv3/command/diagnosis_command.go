package command

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.etcd.io/etcd/etcdctl/v3/diagnosis/engine"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/engine/intf"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/plugins/epstatus"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/plugins/membership"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/plugins/metrics"
	readplugin "go.etcd.io/etcd/etcdctl/v3/diagnosis/plugins/read"
)

var (
	useCluster   bool
	dbQuotaBytes int
	outputFile   string
)

// NewDiagnosisCommand returns the cobra command for "diagnosis".
func NewDiagnosisCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnosis",
		Short: "One-stop etcd diagnosis tool",
		Run:   runDiagnosis,
	}

	cmd.Flags().BoolVar(&useCluster, "cluster", false, "use all endpoints from the cluster member list")
	cmd.Flags().IntVar(&dbQuotaBytes, "etcd-storage-quota-bytes", 2*1024*1024*1024, "etcd storage quota in bytes (the value passed to etcd instance by flag --quota-backend-bytes)")
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
			os.Exit(1)
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
		os.Exit(1)
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
		os.Exit(1)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, report, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write report: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Fprintln(os.Stdout, string(report))
}
