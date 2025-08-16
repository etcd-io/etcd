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
	"bytes"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/v3/mock/mockserver"
)

var grpcMu sync.Mutex

// buildTestRoot builds a minimal root command that mirrors the persistent flags
// expected by clientConfigFromCmd and attaches the alarm subcommands.
func buildTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "etcdctl-test"}

	// Persistent flags required by clientConfigFromCmd and helpers
	root.PersistentFlags().StringSlice("endpoints", []string{"127.0.0.1:2379"}, "")
	root.PersistentFlags().Bool("debug", false, "")
	root.PersistentFlags().StringP("write-out", "w", "simple", "")
	root.PersistentFlags().Bool("hex", false, "")

	root.PersistentFlags().Duration("dial-timeout", 2*time.Second, "")
	root.PersistentFlags().Duration("command-timeout", 5*time.Second, "")
	root.PersistentFlags().Duration("keepalive-time", 2*time.Second, "")
	root.PersistentFlags().Duration("keepalive-timeout", 6*time.Second, "")
	root.PersistentFlags().Int("max-request-bytes", 0, "")
	root.PersistentFlags().Int("max-recv-bytes", 0, "")

	root.PersistentFlags().Bool("insecure-transport", true, "")
	root.PersistentFlags().Bool("insecure-discovery", true, "")
	root.PersistentFlags().Bool("insecure-skip-tls-verify", false, "")
	root.PersistentFlags().String("cert", "", "")
	root.PersistentFlags().String("key", "", "")
	root.PersistentFlags().String("cacert", "", "")

	root.PersistentFlags().String("auth-jwt-token", "", "")
	root.PersistentFlags().String("user", "", "")
	root.PersistentFlags().String("password", "", "")

	root.PersistentFlags().StringP("discovery-srv", "d", "", "")
	root.PersistentFlags().String("discovery-srv-name", "", "")

	// Attach alarm command
	root.AddGroup(&cobra.Group{ID: groupClusterMaintenanceID, Title: "Cluster maintenance commands"})
	root.AddCommand(NewAlarmCommand())
	return root
}

func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		w.Close()
		os.Stdout = old
	}()
	f()
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestAlarmList_PrintsSeededAlarms(t *testing.T) {
	grpcMu.Lock()
	defer grpcMu.Unlock()
	ms, err := mockserver.StartMockServersOnNetwork(1, "tcp")
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer ms.Stop()

	// Seed an alarm
	ms.SetMaintenanceAlarmsAll([]*pb.AlarmMember{{MemberID: 123, Alarm: pb.AlarmType_NOSPACE}})

	root := buildTestRoot()
	root.SetArgs([]string{
		"--endpoints", ms.Servers[0].Address,
		"alarm", "list",
	})

	out := captureStdout(func() {
		_ = root.Execute()
	})

	if !bytes.Contains([]byte(out), []byte("NOSPACE")) {
		t.Fatalf("expected output to contain NOSPACE alarm, got: %q", out)
	}
}

func TestAlarmDisarm_RemovesAlarms(t *testing.T) {
	grpcMu.Lock()
	defer grpcMu.Unlock()
	ms, err := mockserver.StartMockServersOnNetwork(1, "tcp")
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer ms.Stop()

	ms.SetMaintenanceAlarmsAll([]*pb.AlarmMember{{MemberID: 1, Alarm: pb.AlarmType_NOSPACE}})

	root := buildTestRoot()

	// disarm all
	root.SetArgs([]string{"--endpoints", ms.Servers[0].Address, "alarm", "disarm"})
	out := captureStdout(func() { _ = root.Execute() })
	// Expect disarm to print the deactivated alarm
	if !bytes.Contains([]byte(out), []byte("NOSPACE")) {
		t.Fatalf("expected disarm output to mention NOSPACE, got: %q", out)
	}

	// list should now be empty
	root = buildTestRoot()
	root.SetArgs([]string{"--endpoints", ms.Servers[0].Address, "alarm", "list"})
	out = captureStdout(func() { _ = root.Execute() })
	if bytes.Contains([]byte(out), []byte("NOSPACE")) {
		t.Fatalf("expected no NOSPACE after disarm, got: %q", out)
	}
}

func TestAlarmCommands_RejectArgs(t *testing.T) {
	grpcMu.Lock()
	defer grpcMu.Unlock()
	ms, err := mockserver.StartMockServersOnNetwork(1, "tcp")
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer ms.Stop()

	// alarm list should succeed without args
	cmd := buildTestRoot()
	cmd.SetArgs([]string{"--endpoints", ms.Servers[0].Address, "alarm", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected execute error: %v", err)
	}
}
