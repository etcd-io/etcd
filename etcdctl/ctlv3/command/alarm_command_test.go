package command

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/mock/mockserver"
)

// buildTestClient builds clientv3.Client pointing to given mock server
func buildTestClient(t *testing.T, ms *mockserver.MockServer) *clientv3.Client {
	cfg := clientv3.Config{
		Endpoints:            []string{ms.Address},
		DialTimeout:          2 * time.Second,
		DialKeepAliveTime:    2 * time.Second,
		DialKeepAliveTimeout: 6 * time.Second,
		RejectOldCluster:     false,
	}
	cli, err := clientv3.New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return cli
}

func withMockServers(t *testing.T, n int, fn func(ms *mockserver.MockServers)) {
	// Initialize gRPC logger before starting any server goroutines to avoid races
	InitGRPCLoggerForTesting(false)

	ms, err := mockserver.StartMockServers(n)
	if err != nil {
		t.Fatalf("failed to start mock servers: %v", err)
	}
	t.Cleanup(func() { ms.Stop() })
	fn(ms)
}

func attachRequiredFlags(cmd *cobra.Command, endpoint string, timeout time.Duration) {
	// minimal set of flags referenced in clientConfigFromCmd/initDisplayFromCmd
	cmd.Flags().Bool("debug", false, "")
	cmd.Flags().Bool("hex", false, "")
	cmd.Flags().String("write-out", "simple", "")
	cmd.Flags().Duration("command-timeout", timeout, "")
	cmd.Flags().StringSlice("endpoints", []string{endpoint}, "")
	cmd.Flags().Duration("dial-timeout", 2*time.Second, "")
	cmd.Flags().Duration("keepalive-time", 2*time.Second, "")
	cmd.Flags().Duration("keepalive-timeout", 6*time.Second, "")
	cmd.Flags().Int("max-request-bytes", 0, "")
	cmd.Flags().Int("max-recv-bytes", 0, "")
	cmd.Flags().Bool("insecure-transport", true, "")
	cmd.Flags().Bool("insecure-discovery", true, "")
	cmd.Flags().Bool("insecure-skip-tls-verify", false, "")
	cmd.Flags().String("cert", "", "")
	cmd.Flags().String("key", "", "")
	cmd.Flags().String("cacert", "", "")
	cmd.Flags().String("user", "", "")
	cmd.Flags().String("password", "", "")
	cmd.Flags().String("auth-jwt-token", "", "")
	cmd.Flags().String("discovery-srv", "", "")
	cmd.Flags().String("discovery-srv-name", "", "")
}

func TestAlarmList_NoAlarms_PrintsNothing(t *testing.T) {
	withMockServers(t, 1, func(ms *mockserver.MockServers) {
		var buf bytes.Buffer
		oldDisplay := display
		display = NewPrinter("simple", false)
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		t.Cleanup(func() {
			os.Stdout = oldStdout
			display = oldDisplay
		})

		cmd := NewAlarmListCommand()
		attachRequiredFlags(cmd, ms.Servers[0].Address, time.Second)

		alarmListCommandFunc(cmd, []string{})

		w.Close()
		out, _ := io.ReadAll(r)
		buf.Write(out)

		if got := buf.String(); got != "" {
			t.Fatalf("expected no output, got %q", got)
		}
	})
}

func TestAlarmDisarm_DisarmsAll(t *testing.T) {
	withMockServers(t, 1, func(ms *mockserver.MockServers) {
		ms.Servers[0].SetAlarms([]*pb.AlarmMember{{MemberID: 1, Alarm: pb.AlarmType_NOSPACE}})

		var buf bytes.Buffer
		oldDisplay := display
		display = NewPrinter("simple", false)
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		t.Cleanup(func() {
			os.Stdout = oldStdout
			display = oldDisplay
		})

		cmd := NewAlarmDisarmCommand()
		attachRequiredFlags(cmd, ms.Servers[0].Address, time.Second)

		alarmDisarmCommandFunc(cmd, []string{})
		w.Close()
		b, _ := io.ReadAll(r)
		buf.Write(b)

		out := buf.String()
		if out == "" {
			t.Fatalf("expected some output of disarmed alarms, got empty")
		}

		// Create client AFTER command execution to avoid grpclog.SetLoggerV2 race
		cli := buildTestClient(t, ms.Servers[0])
		defer cli.Close()
		resp, err := cli.AlarmList(t.Context())
		if err != nil {
			t.Fatalf("failed AlarmList: %v", err)
		}
		if len(resp.Alarms) != 0 {
			t.Fatalf("expected 0 alarms, got %d", len(resp.Alarms))
		}
	})
}
