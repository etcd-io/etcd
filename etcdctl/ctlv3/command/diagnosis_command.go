package command

import (
	"time"

	"github.com/spf13/cobra"

	"go.etcd.io/etcd/etcdctl/v3/diagnosis/agent"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/engine"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/engine/intf"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/offline"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/plugins/epstatus"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/plugins/membership"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/plugins/metrics"
	readplugin "go.etcd.io/etcd/etcdctl/v3/diagnosis/plugins/read"
)

var diagCfg = agent.GlobalConfig{}

// NewDiagnosisCommand returns the cobra command for "diagnosis".
func NewDiagnosisCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnosis",
		Short: "One-stop etcd diagnosis tool",
		Run:   runDiagnosis,
	}

	cmd.Flags().StringSliceVar(&diagCfg.Endpoints, "endpoints", []string{"127.0.0.1:2379"}, "comma separated etcd endpoints")
	cmd.Flags().BoolVar(&diagCfg.UseClusterEndpoints, "cluster", false, "use all endpoints from the cluster member list")

	cmd.Flags().DurationVar(&diagCfg.DialTimeout, "dial-timeout", 2*time.Second, "dial timeout for client connections")
	cmd.Flags().DurationVar(&diagCfg.CommandTimeout, "command-timeout", 5*time.Second, "command timeout (excluding dial timeout)")
	cmd.Flags().DurationVar(&diagCfg.KeepAliveTime, "keepalive-time", 2*time.Second, "keepalive time for client connections")
	cmd.Flags().DurationVar(&diagCfg.KeepAliveTimeout, "keepalive-timeout", 5*time.Second, "keepalive timeout for client connections")

	cmd.Flags().BoolVar(&diagCfg.Insecure, "insecure-transport", true, "disable transport security for client connections")
	cmd.Flags().BoolVar(&diagCfg.InsecureSkipVerify, "insecure-skip-tls-verify", false, "skip server certificate verification (CAUTION: this option should be enabled only for testing purposes)")
	cmd.Flags().StringVar(&diagCfg.CertFile, "cert", "", "identify secure client using this TLS certificate file")
	cmd.Flags().StringVar(&diagCfg.KeyFile, "key", "", "identify secure client using this TLS key file")
	cmd.Flags().StringVar(&diagCfg.CaFile, "cacert", "", "verify certificates of TLS-enabled secure servers using this CA bundle")

	cmd.Flags().StringVar(&diagCfg.Username, "user", "", "username[:password] for authentication (prompt if password is not supplied)")
	cmd.Flags().StringVar(&diagCfg.Password, "password", "", "password for authentication (if this option is used, --user option shouldn't include password)")
	cmd.Flags().StringVarP(&diagCfg.DNSDomain, "discovery-srv", "d", "", "domain name to query for SRV records describing cluster endpoints")
	cmd.Flags().StringVarP(&diagCfg.DNSService, "discovery-srv-name", "", "", "service name to query when using DNS discovery")
	cmd.Flags().BoolVar(&diagCfg.InsecureDiscovery, "insecure-discovery", true, "accept insecure SRV records describing cluster endpoints")

	cmd.Flags().IntVar(&diagCfg.DbQuotaBytes, "etcd-storage-quota-bytes", 2*1024*1024*1024, "etcd storage quota in bytes (the value passed to etcd instance by flag --quota-backend-bytes)")

	cmd.Flags().BoolVar(&diagCfg.PrintVersion, "version", false, "print the version and exit")

	cmd.Flags().BoolVar(&diagCfg.Offline, "offline", false, "offline analysis")
	cmd.Flags().StringVar(&diagCfg.DataDir, "data-dir", "", "path to data directory")

	return cmd
}

func runDiagnosis(cmd *cobra.Command, args []string) {
	if diagCfg.Offline {
		offline.AnalyzeOffline(diagCfg.DataDir)
		return
	}

	plugins := []intf.Plugin{
		membership.NewPlugin(diagCfg),
		epstatus.NewPlugin(diagCfg),
		readplugin.NewPlugin(diagCfg, false),
		readplugin.NewPlugin(diagCfg, true),
		metrics.NewPlugin(diagCfg),
	}
	engine.Diagnose(diagCfg, plugins)
}
