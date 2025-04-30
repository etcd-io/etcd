// Copyright 2015 The etcd Authors
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
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"go.etcd.io/etcd/pkg/v3/featuregate"
	"go.etcd.io/etcd/pkg/v3/flags"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/features"
)

func TestConfigParsingMemberFlags(t *testing.T) {
	args := []string{
		"-data-dir=testdir",
		"-name=testname",
		"-max-wals=10",
		"-max-snapshots=10",
		"-snapshot-count=10",
		"-experimental-snapshot-catchup-entries=1000",
		"-listen-peer-urls=http://localhost:8000,https://localhost:8001",
		"-listen-client-urls=http://localhost:7000,https://localhost:7001",
		"-listen-client-http-urls=http://localhost:7002,https://localhost:7003",
		// it should be set if -listen-client-urls is set
		"-advertise-client-urls=http://localhost:7000,https://localhost:7001",
	}

	cfg := newConfig()
	err := cfg.parse(args)
	if err != nil {
		t.Fatal(err)
	}

	validateMemberFlags(t, cfg)
}

func TestConfigFileMemberFields(t *testing.T) {
	yc := struct {
		Dir                    string `json:"data-dir"`
		MaxSnapFiles           uint   `json:"max-snapshots"`
		MaxWALFiles            uint   `json:"max-wals"`
		Name                   string `json:"name"`
		SnapshotCount          uint64 `json:"snapshot-count"`
		SnapshotCatchUpEntries uint64 `json:"snapshot-catchup-entries"`
		ListenPeerURLs         string `json:"listen-peer-urls"`
		ListenClientURLs       string `json:"listen-client-urls"`
		ListenClientHTTPURLs   string `json:"listen-client-http-urls"`
		AdvertiseClientURLs    string `json:"advertise-client-urls"`
	}{
		"testdir",
		10,
		10,
		"testname",
		10,
		1000,
		"http://localhost:8000,https://localhost:8001",
		"http://localhost:7000,https://localhost:7001",
		"http://localhost:7002,https://localhost:7003",
		"http://localhost:7000,https://localhost:7001",
	}

	b, err := yaml.Marshal(&yc)
	if err != nil {
		t.Fatal(err)
	}

	tmpfile := mustCreateCfgFile(t, b)
	defer os.Remove(tmpfile.Name())

	args := []string{fmt.Sprintf("--config-file=%s", tmpfile.Name())}

	cfg := newConfig()
	if err = cfg.parse(args); err != nil {
		t.Fatal(err)
	}

	validateMemberFlags(t, cfg)
}

func TestConfigParsingClusteringFlags(t *testing.T) {
	args := []string{
		"-initial-cluster=0=http://localhost:8000",
		"-initial-cluster-state=existing",
		"-initial-cluster-token=etcdtest",
		"-initial-advertise-peer-urls=http://localhost:8000,https://localhost:8001",
		"-advertise-client-urls=http://localhost:7000,https://localhost:7001",
	}

	cfg := newConfig()
	if err := cfg.parse(args); err != nil {
		t.Fatal(err)
	}

	validateClusteringFlags(t, cfg)
}

func TestConfigFileClusteringFields(t *testing.T) {
	yc := struct {
		InitialCluster      string `json:"initial-cluster"`
		ClusterState        string `json:"initial-cluster-state"`
		InitialClusterToken string `json:"initial-cluster-token"`
		AdvertisePeerUrls   string `json:"initial-advertise-peer-urls"`
		AdvertiseClientUrls string `json:"advertise-client-urls"`
	}{
		"0=http://localhost:8000",
		"existing",
		"etcdtest",
		"http://localhost:8000,https://localhost:8001",
		"http://localhost:7000,https://localhost:7001",
	}

	b, err := yaml.Marshal(&yc)
	if err != nil {
		t.Fatal(err)
	}

	tmpfile := mustCreateCfgFile(t, b)
	defer os.Remove(tmpfile.Name())

	args := []string{fmt.Sprintf("--config-file=%s", tmpfile.Name())}
	cfg := newConfig()
	err = cfg.parse(args)
	if err != nil {
		t.Fatal(err)
	}

	validateClusteringFlags(t, cfg)
}

func TestConfigFileClusteringFlags(t *testing.T) {
	tests := []struct {
		Name           string `json:"name"`
		InitialCluster string `json:"initial-cluster"`
		DNSCluster     string `json:"discovery-srv"`
		Durl           string `json:"discovery"`
	}{
		// Use default name and generate a default initial-cluster
		{},
		{
			Name: "non-default",
		},
		{
			InitialCluster: "0=localhost:8000",
		},
		{
			Name:           "non-default",
			InitialCluster: "0=localhost:8000",
		},
		{
			DNSCluster: "example.com",
		},
		{
			Name:       "non-default",
			DNSCluster: "example.com",
		},
		{
			Durl: "http://example.com/abc",
		},
		{
			Name: "non-default",
			Durl: "http://example.com/abc",
		},
	}

	for i, tt := range tests {
		b, err := yaml.Marshal(&tt)
		if err != nil {
			t.Fatal(err)
		}

		tmpfile := mustCreateCfgFile(t, b)
		defer os.Remove(tmpfile.Name())

		args := []string{fmt.Sprintf("--config-file=%s", tmpfile.Name())}

		cfg := newConfig()
		if err := cfg.parse(args); err != nil {
			t.Errorf("%d: err = %v", i, err)
		}
	}
}

func TestConfigParsingConflictClusteringFlags(t *testing.T) {
	conflictArgs := [][]string{
		{
			"-initial-cluster=0=localhost:8000",
			"-discovery=http://example.com/abc",
		},
		{
			"-discovery-srv=example.com",
			"-discovery=http://example.com/abc",
		},
		{
			"-initial-cluster=0=localhost:8000",
			"-discovery-srv=example.com",
		},
		{
			"-initial-cluster=0=localhost:8000",
			"-discovery=http://example.com/abc",
			"-discovery-srv=example.com",
		},
	}

	for i, tt := range conflictArgs {
		cfg := newConfig()
		if err := cfg.parse(tt); !errors.Is(err, embed.ErrConflictBootstrapFlags) {
			t.Errorf("%d: err = %v, want %v", i, err, embed.ErrConflictBootstrapFlags)
		}
	}
}

func TestConfigFileConflictClusteringFlags(t *testing.T) {
	tests := []struct {
		InitialCluster string `json:"initial-cluster"`
		DNSCluster     string `json:"discovery-srv"`
		Durl           string `json:"discovery"`
	}{
		{
			InitialCluster: "0=localhost:8000",
			Durl:           "http://example.com/abc",
		},
		{
			DNSCluster: "example.com",
			Durl:       "http://example.com/abc",
		},
		{
			InitialCluster: "0=localhost:8000",
			DNSCluster:     "example.com",
		},
		{
			InitialCluster: "0=localhost:8000",
			Durl:           "http://example.com/abc",
			DNSCluster:     "example.com",
		},
	}

	for i, tt := range tests {
		b, err := yaml.Marshal(&tt)
		if err != nil {
			t.Fatal(err)
		}

		tmpfile := mustCreateCfgFile(t, b)
		defer os.Remove(tmpfile.Name())

		args := []string{fmt.Sprintf("--config-file=%s", tmpfile.Name())}

		cfg := newConfig()
		if err := cfg.parse(args); !errors.Is(err, embed.ErrConflictBootstrapFlags) {
			t.Errorf("%d: err = %v, want %v", i, err, embed.ErrConflictBootstrapFlags)
		}
	}
}

func TestConfigParsingMissedAdvertiseClientURLsFlag(t *testing.T) {
	tests := []struct {
		args []string
		werr error
	}{
		{
			[]string{
				"-initial-cluster=infra1=http://127.0.0.1:2380",
				"-listen-client-urls=http://127.0.0.1:2379",
			},
			embed.ErrUnsetAdvertiseClientURLsFlag,
		},
		{
			[]string{
				"-discovery-srv=example.com",
				"-listen-client-urls=http://127.0.0.1:2379",
			},
			embed.ErrUnsetAdvertiseClientURLsFlag,
		},
		{
			[]string{
				"-discovery=http://example.com/abc",
				"-discovery-fallback=exit",
				"-listen-client-urls=http://127.0.0.1:2379",
			},
			embed.ErrUnsetAdvertiseClientURLsFlag,
		},
		{
			[]string{
				"-listen-client-urls=http://127.0.0.1:2379",
			},
			embed.ErrUnsetAdvertiseClientURLsFlag,
		},
	}

	for i, tt := range tests {
		cfg := newConfig()
		if err := cfg.parse(tt.args); !errors.Is(err, tt.werr) {
			t.Errorf("%d: err = %v, want %v", i, err, tt.werr)
		}
	}
}

// TestExperimentalSnapshotCatchUpEntriesFlagMigration tests the migration from
// --experimental-snapshot-catch-up-entries to --snapshot-catch-up-entries
func TestExperimentalSnapshotCatchUpEntriesFlagMigration(t *testing.T) {
	testCases := []struct {
		name                               string
		snapshotCatchUpEntries             uint64
		experimentalSnapshotCatchUpEntries uint64
		wantErr                            bool
		wantConfig                         uint64
	}{
		{
			name:       "default",
			wantConfig: etcdserver.DefaultSnapshotCatchUpEntries,
		},
		{
			name:                               "cannot set both experimental flag and non experimental flag",
			experimentalSnapshotCatchUpEntries: 1000,
			snapshotCatchUpEntries:             2000,
			wantErr:                            true,
		},
		{
			name:                               "can set experimental flag",
			experimentalSnapshotCatchUpEntries: 1000,
			wantConfig:                         1000,
		},
		{
			name:                   "can set non-experimental flag",
			snapshotCatchUpEntries: 2000,
			wantConfig:             2000,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdLineArgs := []string{}
			yc := struct {
				ExperimentalSnapshotCatchUpEntries uint64 `json:"experimental-snapshot-catch-up-entries,omitempty"`
				SnapshotCatchUpEntries             uint64 `json:"snapshot-catchup-entries,omitempty"`
			}{}

			if tc.snapshotCatchUpEntries > 0 {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--snapshot-catchup-entries=%d", tc.snapshotCatchUpEntries))
				yc.SnapshotCatchUpEntries = tc.snapshotCatchUpEntries
			}

			if tc.experimentalSnapshotCatchUpEntries > 0 {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-snapshot-catchup-entries=%d", tc.experimentalSnapshotCatchUpEntries))
				yc.ExperimentalSnapshotCatchUpEntries = tc.experimentalSnapshotCatchUpEntries
			}

			cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile := generateCfgsFromFileAndCmdLine(t, yc, cmdLineArgs)

			if tc.wantErr {
				if errFromCmdLine == nil || errFromFile == nil {
					t.Fatal("expect parse error")
				}
				return
			}
			if errFromCmdLine != nil || errFromFile != nil {
				t.Fatal("error parsing config")
			}

			require.Equal(t, tc.wantConfig, cfgFromCmdLine.ec.SnapshotCatchUpEntries)
			require.Equal(t, tc.wantConfig, cfgFromFile.ec.SnapshotCatchUpEntries)
		})
	}
}

func TestConfigIsNewCluster(t *testing.T) {
	tests := []struct {
		state  string
		wIsNew bool
	}{
		{embed.ClusterStateFlagExisting, false},
		{embed.ClusterStateFlagNew, true},
	}
	for i, tt := range tests {
		cfg := newConfig()
		args := []string{"--initial-cluster-state", tests[i].state}
		err := cfg.parse(args)
		require.NoErrorf(t, err, "#%d: unexpected clusterState.Set error: %v", i, err)
		if g := cfg.ec.IsNewCluster(); g != tt.wIsNew {
			t.Errorf("#%d: isNewCluster = %v, want %v", i, g, tt.wIsNew)
		}
	}
}

func TestConfigFileElectionTimeout(t *testing.T) {
	tests := []struct {
		TickMs     uint `json:"heartbeat-interval"`
		ElectionMs uint `json:"election-timeout"`
		errStr     string
	}{
		{
			ElectionMs: 1000,
			TickMs:     800,
			errStr:     "should be at least as 5 times as",
		},
		{
			ElectionMs: 60000,
			TickMs:     10000,
			errStr:     "is too long, and should be set less than",
		},
		{
			ElectionMs: 100,
			TickMs:     0,
			errStr:     "--heartbeat-interval must be >0 (set to 0ms)",
		},
		{
			ElectionMs: 0,
			TickMs:     100,
			errStr:     "--election-timeout must be >0 (set to 0ms)",
		},
	}

	for i, tt := range tests {
		b, err := yaml.Marshal(&tt)
		if err != nil {
			t.Fatal(err)
		}

		tmpfile := mustCreateCfgFile(t, b)
		defer os.Remove(tmpfile.Name())

		args := []string{fmt.Sprintf("--config-file=%s", tmpfile.Name())}

		cfg := newConfig()
		if err := cfg.parse(args); err == nil || !strings.Contains(err.Error(), tt.errStr) {
			t.Errorf("%d: Wrong err = %v", i, err)
		}
	}
}

func TestFlagsPresentInHelp(t *testing.T) {
	cfg := newConfig()
	cfg.cf.flagSet.VisitAll(func(f *flag.Flag) {
		if _, ok := f.Value.(*flags.IgnoredFlag); ok {
			// Ignored flags do not need to be in the help
			return
		}

		flagText := fmt.Sprintf("--%s", f.Name)
		if !strings.Contains(flagsline, flagText) && !strings.Contains(usageline, flagText) {
			t.Errorf("Neither flagsline nor usageline in help.go contains flag named %s", flagText)
		}
	})
}

func TestParseFeatureGateFlags(t *testing.T) {
	testCases := []struct {
		name             string
		args             []string
		expectErr        bool
		expectedFeatures map[featuregate.Feature]bool
	}{
		{
			name: "default",
			expectedFeatures: map[featuregate.Feature]bool{
				features.StopGRPCServiceOnDefrag: false,
			},
		},
		{
			name: "cannot set both experimental flag and feature gate flag",
			args: []string{
				"--experimental-stop-grpc-service-on-defrag=false",
				"--feature-gates=StopGRPCServiceOnDefrag=true",
			},
			expectErr: true,
		},
		{
			name: "ok to set different experimental flag and feature gate flag",
			args: []string{
				"--experimental-stop-grpc-service-on-defrag=true",
				"--feature-gates=InitialCorruptCheck=true",
			},
			expectedFeatures: map[featuregate.Feature]bool{
				features.StopGRPCServiceOnDefrag: true,
				features.InitialCorruptCheck:     true,
			},
		},
		{
			name: "can set feature gate from experimental flag",
			args: []string{
				"--experimental-stop-grpc-service-on-defrag=true",
			},
			expectedFeatures: map[featuregate.Feature]bool{
				features.StopGRPCServiceOnDefrag: true,
				features.InitialCorruptCheck:     false,
			},
		},
		{
			name: "can set feature gate from feature gate flag",
			args: []string{
				"--feature-gates=StopGRPCServiceOnDefrag=true,InitialCorruptCheck=true",
			},
			expectedFeatures: map[featuregate.Feature]bool{
				features.StopGRPCServiceOnDefrag: true,
				features.InitialCorruptCheck:     true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newConfig()
			err := cfg.parse(tc.args)
			if tc.expectErr {
				require.Errorf(t, err, "expect parse error")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for k, v := range tc.expectedFeatures {
				if cfg.ec.ServerFeatureGate.Enabled(k) != v {
					t.Errorf("expected feature gate %s=%v, got %v", k, v, cfg.ec.ServerFeatureGate.Enabled(k))
				}
			}
		})
	}
}

// TestDowngradeCheckTimeFlagMigration tests the migration from
// --experimental-downgrade-check-time to --downgrade-check-time
func TestDowngradeCheckTimeFlagMigration(t *testing.T) {
	testCases := []struct {
		name                           string
		downgradeCheckTime             string
		experimentalDowngradeCheckTime string
		wantErr                        bool
		wantConfig                     time.Duration
	}{
		{
			name:       "default",
			wantConfig: embed.DefaultDowngradeCheckTime,
		},
		{
			name:                           "cannot set both experimental flag and non experimental flag",
			experimentalDowngradeCheckTime: "1m",
			downgradeCheckTime:             "2m",
			wantErr:                        true,
		},
		{
			name:                           "can set experimental flag",
			experimentalDowngradeCheckTime: "1m",
			wantConfig:                     time.Minute,
		},
		{
			name:               "can set non-experimental flag",
			downgradeCheckTime: "2m",
			wantConfig:         2 * time.Minute,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdLineArgs := []string{}
			yc := struct {
				ExperimentalDowngradeCheckTime time.Duration `json:"experimental-downgrade-check-time,omitempty"`
				DowngradeCheckTime             time.Duration `json:"downgrade-check-time,omitempty"`
			}{}

			if tc.downgradeCheckTime != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--downgrade-check-time=%s", tc.downgradeCheckTime))
				downgradeCheckTime, err := time.ParseDuration(tc.downgradeCheckTime)
				require.NoError(t, err)
				yc.DowngradeCheckTime = downgradeCheckTime
			}

			if tc.experimentalDowngradeCheckTime != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-downgrade-check-time=%s", tc.experimentalDowngradeCheckTime))
				experimentalDowngradeCheckTime, err := time.ParseDuration(tc.experimentalDowngradeCheckTime)
				require.NoError(t, err)
				yc.ExperimentalDowngradeCheckTime = experimentalDowngradeCheckTime
			}

			cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile := generateCfgsFromFileAndCmdLine(t, yc, cmdLineArgs)

			if tc.wantErr {
				if errFromCmdLine == nil || errFromFile == nil {
					t.Fatal("expect parse error")
				}
				return
			}
			if errFromCmdLine != nil || errFromFile != nil {
				t.Fatal("error parsing config")
			}

			require.Equal(t, tc.wantConfig, cfgFromCmdLine.ec.DowngradeCheckTime)
			require.Equal(t, tc.wantConfig, cfgFromFile.ec.DowngradeCheckTime)
		})
	}
}

// TestCompactHashCheckTimeFlagMigration tests the migration from
// --experimental-compact-hash-check-time to --compact-hash-check-time
// TODO: delete in v3.7
func TestCompactHashCheckTimeFlagMigration(t *testing.T) {
	testCases := []struct {
		name                             string
		compactHashCheckTime             string
		experimentalCompactHashCheckTime string
		expectErr                        bool
		expectedCompactHashCheckTime     time.Duration
	}{
		{
			name:                         "default",
			expectedCompactHashCheckTime: time.Minute,
		},
		{
			name:                             "cannot set both experimental flag and non experimental flag",
			compactHashCheckTime:             "2m",
			experimentalCompactHashCheckTime: "3m",
			expectErr:                        true,
		},
		{
			name:                             "can set experimental flag",
			experimentalCompactHashCheckTime: "3m",
			expectedCompactHashCheckTime:     3 * time.Minute,
		},
		{
			name:                         "can set non experimental flag",
			compactHashCheckTime:         "2m",
			expectedCompactHashCheckTime: 2 * time.Minute,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdLineArgs := []string{}
			yc := struct {
				ExperimentalCompactHashCheckTime time.Duration `json:"experimental-compact-hash-check-time,omitempty"`
				CompactHashCheckTime             time.Duration `json:"compact-hash-check-time,omitempty"`
			}{}

			if tc.compactHashCheckTime != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--compact-hash-check-time=%s", tc.compactHashCheckTime))
				compactHashCheckTime, err := time.ParseDuration(tc.compactHashCheckTime)
				require.NoError(t, err)
				yc.CompactHashCheckTime = compactHashCheckTime
			}

			if tc.experimentalCompactHashCheckTime != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-compact-hash-check-time=%s", tc.experimentalCompactHashCheckTime))
				experimentalCompactHashCheckTime, err := time.ParseDuration(tc.experimentalCompactHashCheckTime)
				require.NoError(t, err)
				yc.ExperimentalCompactHashCheckTime = experimentalCompactHashCheckTime
			}

			cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile := generateCfgsFromFileAndCmdLine(t, yc, cmdLineArgs)

			if tc.expectErr {
				if errFromCmdLine == nil || errFromFile == nil {
					t.Fatal("expect parse error")
				}
				return
			}
			if errFromCmdLine != nil || errFromFile != nil {
				t.Fatal("error parsing config")
			}

			require.Equal(t, tc.expectedCompactHashCheckTime, cfgFromCmdLine.ec.CompactHashCheckTime)
			require.Equal(t, tc.expectedCompactHashCheckTime, cfgFromFile.ec.CompactHashCheckTime)
		})
	}
}

// TestCompactionSleepIntervalFlagMigration tests the migration from
// --experimental-compaction-sleep-interval to --compaction-sleep-interval
func TestCompactionSleepIntervalFlagMigration(t *testing.T) {
	testCases := []struct {
		name                                string
		compactionSleepInterval             string
		experimentalCompactionSleepInterval string
		wantErr                             bool
		wantConfig                          time.Duration
	}{
		{
			name:       "default",
			wantConfig: time.Duration(0),
		},
		{
			name:                                "cannot set both experimental flag and non experimental flag",
			experimentalCompactionSleepInterval: "30s",
			compactionSleepInterval:             "15s",
			wantErr:                             true,
		},
		{
			name:                                "can set experimental flag",
			experimentalCompactionSleepInterval: "30s",
			wantConfig:                          30 * time.Second,
		},
		{
			name:                    "can set non-experimental flag",
			compactionSleepInterval: "1m",
			wantConfig:              time.Minute,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdLineArgs := []string{}
			yc := struct {
				ExperimentalCompactionSleepInterval time.Duration `json:"experimental-compaction-sleep-interval,omitempty"`
				CompactionSleepInterval             time.Duration `json:"compaction-sleep-interval,omitempty"`
			}{}

			if tc.compactionSleepInterval != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--compaction-sleep-interval=%s", tc.compactionSleepInterval))
				compactionSleepInterval, err := time.ParseDuration(tc.compactionSleepInterval)
				require.NoError(t, err)
				yc.CompactionSleepInterval = compactionSleepInterval
			}

			if tc.experimentalCompactionSleepInterval != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-compaction-sleep-interval=%s", tc.experimentalCompactionSleepInterval))
				experimentalCompactionSleepInterval, err := time.ParseDuration(tc.experimentalCompactionSleepInterval)
				require.NoError(t, err)
				yc.ExperimentalCompactionSleepInterval = experimentalCompactionSleepInterval
			}

			cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile := generateCfgsFromFileAndCmdLine(t, yc, cmdLineArgs)

			if tc.wantErr {
				if errFromCmdLine == nil || errFromFile == nil {
					t.Fatal("expect parse error")
				}
				return
			}
			if errFromCmdLine != nil || errFromFile != nil {
				t.Fatal("error parsing config")
			}

			require.Equal(t, tc.wantConfig, cfgFromCmdLine.ec.CompactionSleepInterval)
			require.Equal(t, tc.wantConfig, cfgFromFile.ec.CompactionSleepInterval)
		})
	}
}

// TestCorruptCheckTimeFlagMigration tests the migration from
// --experimental-corrupt-check-time to --corrupt-check-time
// TODO: delete in v3.7
func TestCorruptCheckTimeFlagMigration(t *testing.T) {
	testCases := []struct {
		name                         string
		corruptCheckTime             string
		experimentalCorruptCheckTime string
		expectErr                    bool
		expectedCorruptCheckTime     time.Duration
	}{
		{
			name:                         "cannot set both experimental flag and non experimental flag",
			corruptCheckTime:             "2m",
			experimentalCorruptCheckTime: "3m",
			expectErr:                    true,
		},
		{
			name:                         "can set experimental flag",
			experimentalCorruptCheckTime: "3m",
			expectedCorruptCheckTime:     3 * time.Minute,
		},
		{
			name:                     "can set non experimental flag",
			corruptCheckTime:         "2m",
			expectedCorruptCheckTime: 2 * time.Minute,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdLineArgs := []string{}
			yc := struct {
				ExperimentalCorruptCheckTime time.Duration `json:"experimental-corrupt-check-time,omitempty"`
				CorruptCheckTime             time.Duration `json:"corrupt-check-time,omitempty"`
			}{}

			if tc.corruptCheckTime != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--corrupt-check-time=%s", tc.corruptCheckTime))
				corruptCheckTime, err := time.ParseDuration(tc.corruptCheckTime)
				require.NoError(t, err)
				yc.CorruptCheckTime = corruptCheckTime
			}

			if tc.experimentalCorruptCheckTime != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-corrupt-check-time=%s", tc.experimentalCorruptCheckTime))
				experimentalCorruptCheckTime, err := time.ParseDuration(tc.experimentalCorruptCheckTime)
				require.NoError(t, err)
				yc.ExperimentalCorruptCheckTime = experimentalCorruptCheckTime
			}

			cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile := generateCfgsFromFileAndCmdLine(t, yc, cmdLineArgs)

			if tc.expectErr {
				if errFromCmdLine == nil || errFromFile == nil {
					t.Fatal("expect parse error")
				}
				return
			}
			if errFromCmdLine != nil || errFromFile != nil {
				t.Fatal("error parsing config")
			}

			require.Equal(t, tc.expectedCorruptCheckTime, cfgFromCmdLine.ec.CorruptCheckTime)
			require.Equal(t, tc.expectedCorruptCheckTime, cfgFromFile.ec.CorruptCheckTime)
		})
	}
}

// TestCompactionBatchLimitFlagMigration tests the migration from
// --experimental-compaction-batch-limit to --compaction-batch-limit
// TODO: delete in v3.7
func TestCompactionBatchLimitFlagMigration(t *testing.T) {
	testCases := []struct {
		name                             string
		compactionBatchLimit             int
		experimentalCompactionBatchLimit int
		expectErr                        bool
		expectedCompactionBatchLimit     int
	}{
		{
			name:                             "cannot set both experimental flag and non experimental flag",
			compactionBatchLimit:             1,
			experimentalCompactionBatchLimit: 2,
			expectErr:                        true,
		},
		{
			name:                             "can set experimental flag",
			experimentalCompactionBatchLimit: 2,
			expectedCompactionBatchLimit:     2,
		},
		{
			name:                         "can set non experimental flag",
			compactionBatchLimit:         1,
			expectedCompactionBatchLimit: 1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdLineArgs := []string{}
			yc := struct {
				ExperimentalCompactionBatchLimit int `json:"experimental-compaction-batch-limit,omitempty"`
				CompactionBatchLimit             int `json:"compaction-batch-limit,omitempty"`
			}{}

			if tc.compactionBatchLimit != 0 {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--compaction-batch-limit=%d", tc.compactionBatchLimit))
				yc.CompactionBatchLimit = tc.compactionBatchLimit
			}

			if tc.experimentalCompactionBatchLimit != 0 {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-compaction-batch-limit=%d", tc.experimentalCompactionBatchLimit))
				yc.ExperimentalCompactionBatchLimit = tc.experimentalCompactionBatchLimit
			}

			cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile := generateCfgsFromFileAndCmdLine(t, yc, cmdLineArgs)

			if tc.expectErr {
				if errFromCmdLine == nil || errFromFile == nil {
					t.Fatal("expect parse error")
				}
				return
			}
			if errFromCmdLine != nil || errFromFile != nil {
				t.Fatal("error parsing config")
			}

			require.Equal(t, tc.expectedCompactionBatchLimit, cfgFromCmdLine.ec.CompactionBatchLimit)
			require.Equal(t, tc.expectedCompactionBatchLimit, cfgFromFile.ec.CompactionBatchLimit)
		})
	}
}

// TestWatchProgressNotifyInterval tests the migration from
// --experimental-watch-progress-notify-interval to --watch-progress-notify-interval
// TODO: delete in v3.7
func TestWatchProgressNotifyInterval(t *testing.T) {
	testCases := []struct {
		name                                    string
		watchProgressNotifyInterval             string
		experimentalWatchProgressNotifyInterval string
		expectErr                               bool
		expectedWatchProgressNotifyInterval     time.Duration
	}{
		{
			name:                                    "cannot set both experimental flag and non experimental flag",
			watchProgressNotifyInterval:             "2m",
			experimentalWatchProgressNotifyInterval: "3m",
			expectErr:                               true,
		},
		{
			name:                                    "can set experimental flag",
			experimentalWatchProgressNotifyInterval: "3m",
			expectedWatchProgressNotifyInterval:     3 * time.Minute,
		},
		{
			name:                                "can set non experimental flag",
			watchProgressNotifyInterval:         "2m",
			expectedWatchProgressNotifyInterval: 2 * time.Minute,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdLineArgs := []string{}
			yc := struct {
				ExperimentalWatchProgressNotifyInterval time.Duration `json:"experimental-watch-progress-notify-interval,omitempty"`
				WatchProgressNotifyInterval             time.Duration `json:"watch-progress-notify-interval,omitempty"`
			}{}

			if tc.watchProgressNotifyInterval != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--watch-progress-notify-interval=%s", tc.watchProgressNotifyInterval))
				watchProgressNotifyInterval, err := time.ParseDuration(tc.watchProgressNotifyInterval)
				require.NoError(t, err)
				yc.WatchProgressNotifyInterval = watchProgressNotifyInterval
			}

			if tc.experimentalWatchProgressNotifyInterval != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-watch-progress-notify-interval=%s", tc.experimentalWatchProgressNotifyInterval))
				experimentalWatchProgressNotifyInterval, err := time.ParseDuration(tc.experimentalWatchProgressNotifyInterval)
				require.NoError(t, err)
				yc.ExperimentalWatchProgressNotifyInterval = experimentalWatchProgressNotifyInterval
			}

			cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile := generateCfgsFromFileAndCmdLine(t, yc, cmdLineArgs)

			if tc.expectErr {
				if errFromCmdLine == nil || errFromFile == nil {
					t.Fatal("expect parse error")
				}
				return
			}
			if errFromCmdLine != nil || errFromFile != nil {
				t.Fatal("error parsing config")
			}

			require.Equal(t, tc.expectedWatchProgressNotifyInterval, cfgFromCmdLine.ec.WatchProgressNotifyInterval)
			require.Equal(t, tc.expectedWatchProgressNotifyInterval, cfgFromFile.ec.WatchProgressNotifyInterval)
		})
	}
}

// TestWarningApplyDuration tests the migration from
// --experimental-warning-apply-duration to --warning-apply-duration
// TODO: delete in v3.7
func TestWarningApplyDuration(t *testing.T) {
	testCases := []struct {
		name                             string
		warningApplyDuration             string
		experimentalWarningApplyDuration string
		expectErr                        bool
		expectedWarningApplyDuration     time.Duration
	}{
		{
			name:                             "cannot set both experimental flag and non experimental flag",
			warningApplyDuration:             "2m",
			experimentalWarningApplyDuration: "3m",
			expectErr:                        true,
		},
		{
			name:                             "can set experimental flag",
			experimentalWarningApplyDuration: "3m",
			expectedWarningApplyDuration:     3 * time.Minute,
		},
		{
			name:                         "can set non experimental flag",
			warningApplyDuration:         "2m",
			expectedWarningApplyDuration: 2 * time.Minute,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdLineArgs := []string{}
			yc := struct {
				ExperimentalWarningApplyDuration time.Duration `json:"experimental-warning-apply-duration,omitempty"`
				WarningApplyDuration             time.Duration `json:"warning-apply-duration,omitempty"`
			}{}

			if tc.warningApplyDuration != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--warning-apply-duration=%s", tc.warningApplyDuration))
				warningApplyDuration, err := time.ParseDuration(tc.warningApplyDuration)
				require.NoError(t, err)
				yc.WarningApplyDuration = warningApplyDuration
			}

			if tc.experimentalWarningApplyDuration != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-warning-apply-duration=%s", tc.experimentalWarningApplyDuration))
				experimentalWarningApplyDuration, err := time.ParseDuration(tc.experimentalWarningApplyDuration)
				require.NoError(t, err)
				yc.ExperimentalWarningApplyDuration = experimentalWarningApplyDuration
			}

			cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile := generateCfgsFromFileAndCmdLine(t, yc, cmdLineArgs)

			if tc.expectErr {
				if errFromCmdLine == nil || errFromFile == nil {
					t.Fatal("expect parse error")
				}
				return
			}
			if errFromCmdLine != nil || errFromFile != nil {
				t.Fatal("error parsing config")
			}

			require.Equal(t, tc.expectedWarningApplyDuration, cfgFromCmdLine.ec.WarningApplyDuration)
			require.Equal(t, tc.expectedWarningApplyDuration, cfgFromFile.ec.WarningApplyDuration)
		})
	}
}

// TestBootstrapDefragThresholdMegabytesFlagMigration tests the migration from
// --experimental-bootstrap-defrag-threshold-megabytes to --bootstrap-defrag-threshold-megabytes
// TODO: delete in v3.7
func TestBootstrapDefragThresholdMegabytesFlagMigration(t *testing.T) {
	testCases := []struct {
		name                                          string
		bootstrapDefragThresholdMegabytes             uint
		experimentalBootstrapDefragThresholdMegabytes uint
		expectErr                                     bool
		expectedBootstrapDefragThresholdMegabytes     uint
	}{
		{
			name:                              "cannot set both experimental flag and non experimental flag",
			bootstrapDefragThresholdMegabytes: 100,
			experimentalBootstrapDefragThresholdMegabytes: 200,
			expectErr: true,
		},
		{
			name: "can set experimental flag",
			experimentalBootstrapDefragThresholdMegabytes: 200,
			expectedBootstrapDefragThresholdMegabytes:     200,
		},
		{
			name:                              "can set non experimental flag",
			bootstrapDefragThresholdMegabytes: 100,
			expectedBootstrapDefragThresholdMegabytes: 100,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdLineArgs := []string{}
			yc := struct {
				ExperimentalBootstrapDefragThresholdMegabytes uint `json:"experimental-bootstrap-defrag-threshold-megabytes,omitempty"`
				BootstrapDefragThresholdMegabytes             uint `json:"bootstrap-defrag-threshold-megabytes,omitempty"`
			}{}

			if tc.bootstrapDefragThresholdMegabytes != 0 {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--bootstrap-defrag-threshold-megabytes=%d", tc.bootstrapDefragThresholdMegabytes))
				yc.BootstrapDefragThresholdMegabytes = tc.bootstrapDefragThresholdMegabytes
			}

			if tc.experimentalBootstrapDefragThresholdMegabytes != 0 {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-bootstrap-defrag-threshold-megabytes=%d", tc.experimentalBootstrapDefragThresholdMegabytes))
				yc.ExperimentalBootstrapDefragThresholdMegabytes = tc.experimentalBootstrapDefragThresholdMegabytes
			}

			cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile := generateCfgsFromFileAndCmdLine(t, yc, cmdLineArgs)

			if tc.expectErr {
				if errFromCmdLine == nil || errFromFile == nil {
					t.Fatal("expect parse error")
				}
				return
			}
			if errFromCmdLine != nil || errFromFile != nil {
				t.Fatal("error parsing config")
			}

			require.Equal(t, tc.expectedBootstrapDefragThresholdMegabytes, cfgFromCmdLine.ec.BootstrapDefragThresholdMegabytes)
			require.Equal(t, tc.expectedBootstrapDefragThresholdMegabytes, cfgFromFile.ec.BootstrapDefragThresholdMegabytes)
		})
	}
}

// TestMemoryMlockFlagMigration tests the migration from
// --experimental-memory-mlock to --memory-mlock
// TODO: delete in v3.7
func TestMemoryMlockFlagMigration(t *testing.T) {
	testCases := []struct {
		name                    string
		memoryMlock             bool
		experimentalMemoryMlock bool
		expectedMemoryMlock     bool
		expectErr               bool
	}{
		{
			name:                "default",
			expectedMemoryMlock: false,
		},
		{
			name:                    "cannot set both experimental flag and non experimental flag",
			memoryMlock:             true,
			experimentalMemoryMlock: true,
			expectErr:               true,
		},
		{
			name:                    "can set experimental flag",
			experimentalMemoryMlock: true,
			expectedMemoryMlock:     true,
		},
		{
			name:                "can set non experimental flag",
			memoryMlock:         true,
			expectedMemoryMlock: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdLineArgs := []string{}
			yc := struct {
				MemoryMlock             bool `json:"memory-mlock,omitempty"`
				ExperimentalMemoryMlock bool `json:"experimental-memory-mlock,omitempty"`
			}{}

			if tc.memoryMlock {
				cmdLineArgs = append(cmdLineArgs, "--memory-mlock")
				yc.MemoryMlock = tc.memoryMlock
			}

			if tc.experimentalMemoryMlock {
				cmdLineArgs = append(cmdLineArgs, "--experimental-memory-mlock")
				yc.ExperimentalMemoryMlock = tc.experimentalMemoryMlock
			}

			cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile := generateCfgsFromFileAndCmdLine(t, yc, cmdLineArgs)

			if tc.expectErr {
				if errFromCmdLine == nil || errFromFile == nil {
					t.Fatal("expect parse error")
				}
				return
			}

			if errFromCmdLine != nil || errFromFile != nil {
				t.Fatal("error parsing config")
			}

			if cfgFromCmdLine.ec.MemoryMlock != tc.expectedMemoryMlock {
				t.Errorf("expected MemoryMlock=%v, got %v", tc.expectedMemoryMlock, cfgFromCmdLine.ec.MemoryMlock)
			}
			if cfgFromFile.ec.MemoryMlock != tc.expectedMemoryMlock {
				t.Errorf("expected MemoryMlock=%v, got %v", tc.expectedMemoryMlock, cfgFromFile.ec.MemoryMlock)
			}
		})
	}
}

// TODO delete in v3.7
func generateCfgsFromFileAndCmdLine(t *testing.T, yc any, cmdLineArgs []string) (*config, error, *config, error) {
	b, err := yaml.Marshal(&yc)
	require.NoError(t, err)

	tmpfile := mustCreateCfgFile(t, b)
	defer os.Remove(tmpfile.Name())

	cfgFromCmdLine := newConfig()
	errFromCmdLine := cfgFromCmdLine.parse(cmdLineArgs)

	cfgFromFile := newConfig()
	errFromFile := cfgFromFile.parse([]string{fmt.Sprintf("--config-file=%s", tmpfile.Name())})
	return cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile
}

func mustCreateCfgFile(t *testing.T, b []byte) *os.File {
	tmpfile, err := os.CreateTemp("", "servercfg")
	if err != nil {
		t.Fatal(err)
	}

	_, err = tmpfile.Write(b)
	if err != nil {
		t.Fatal(err)
	}
	err = tmpfile.Close()
	if err != nil {
		t.Fatal(err)
	}

	return tmpfile
}

func validateMemberFlags(t *testing.T, cfg *config) {
	wcfg := &embed.Config{
		Dir:                    "testdir",
		ListenPeerUrls:         []url.URL{{Scheme: "http", Host: "localhost:8000"}, {Scheme: "https", Host: "localhost:8001"}},
		ListenClientUrls:       []url.URL{{Scheme: "http", Host: "localhost:7000"}, {Scheme: "https", Host: "localhost:7001"}},
		ListenClientHttpUrls:   []url.URL{{Scheme: "http", Host: "localhost:7002"}, {Scheme: "https", Host: "localhost:7003"}},
		MaxSnapFiles:           10,
		MaxWalFiles:            10,
		Name:                   "testname",
		SnapshotCount:          10,
		SnapshotCatchUpEntries: 1000,
	}

	if cfg.ec.Dir != wcfg.Dir {
		t.Errorf("dir = %v, want %v", cfg.ec.Dir, wcfg.Dir)
	}
	if cfg.ec.MaxSnapFiles != wcfg.MaxSnapFiles {
		t.Errorf("maxsnap = %v, want %v", cfg.ec.MaxSnapFiles, wcfg.MaxSnapFiles)
	}
	if cfg.ec.MaxWalFiles != wcfg.MaxWalFiles {
		t.Errorf("maxwal = %v, want %v", cfg.ec.MaxWalFiles, wcfg.MaxWalFiles)
	}
	if cfg.ec.Name != wcfg.Name {
		t.Errorf("name = %v, want %v", cfg.ec.Name, wcfg.Name)
	}
	if cfg.ec.SnapshotCount != wcfg.SnapshotCount {
		t.Errorf("snapcount = %v, want %v", cfg.ec.SnapshotCount, wcfg.SnapshotCount)
	}
	if cfg.ec.SnapshotCatchUpEntries != wcfg.SnapshotCatchUpEntries {
		t.Errorf("snapshot catch up entries = %v, want %v", cfg.ec.SnapshotCatchUpEntries, wcfg.SnapshotCatchUpEntries)
	}
	if !reflect.DeepEqual(cfg.ec.ListenPeerUrls, wcfg.ListenPeerUrls) {
		t.Errorf("listen-peer-urls = %v, want %v", cfg.ec.ListenPeerUrls, wcfg.ListenPeerUrls)
	}
	if !reflect.DeepEqual(cfg.ec.ListenClientUrls, wcfg.ListenClientUrls) {
		t.Errorf("listen-client-urls = %v, want %v", cfg.ec.ListenClientUrls, wcfg.ListenClientUrls)
	}
	if !reflect.DeepEqual(cfg.ec.ListenClientHttpUrls, wcfg.ListenClientHttpUrls) {
		t.Errorf("listen-client-http-urls = %v, want %v", cfg.ec.ListenClientHttpUrls, wcfg.ListenClientHttpUrls)
	}
}

func validateClusteringFlags(t *testing.T, cfg *config) {
	wcfg := newConfig()
	wcfg.ec.AdvertisePeerUrls = []url.URL{{Scheme: "http", Host: "localhost:8000"}, {Scheme: "https", Host: "localhost:8001"}}
	wcfg.ec.AdvertiseClientUrls = []url.URL{{Scheme: "http", Host: "localhost:7000"}, {Scheme: "https", Host: "localhost:7001"}}
	wcfg.ec.ClusterState = embed.ClusterStateFlagExisting
	wcfg.ec.InitialCluster = "0=http://localhost:8000"
	wcfg.ec.InitialClusterToken = "etcdtest"

	if cfg.ec.ClusterState != wcfg.ec.ClusterState {
		t.Errorf("clusterState = %v, want %v", cfg.ec.ClusterState, wcfg.ec.ClusterState)
	}
	if cfg.ec.InitialCluster != wcfg.ec.InitialCluster {
		t.Errorf("initialCluster = %v, want %v", cfg.ec.InitialCluster, wcfg.ec.InitialCluster)
	}
	if cfg.ec.InitialClusterToken != wcfg.ec.InitialClusterToken {
		t.Errorf("initialClusterToken = %v, want %v", cfg.ec.InitialClusterToken, wcfg.ec.InitialClusterToken)
	}
	if !reflect.DeepEqual(cfg.ec.AdvertisePeerUrls, wcfg.ec.AdvertisePeerUrls) {
		t.Errorf("initial-advertise-peer-urls = %v, want %v", cfg.ec.AdvertisePeerUrls, wcfg.ec.AdvertisePeerUrls)
	}
	if !reflect.DeepEqual(cfg.ec.AdvertiseClientUrls, wcfg.ec.AdvertiseClientUrls) {
		t.Errorf("advertise-client-urls = %v, want %v", cfg.ec.AdvertiseClientUrls, wcfg.ec.AdvertiseClientUrls)
	}
}

func TestConfigFileDeprecatedOptions(t *testing.T) {
	// Define a minimal config struct with only the fields we need
	type configFileYAML struct {
		SnapshotCount                                 uint64        `json:"snapshot-count,omitempty"`
		MaxSnapFiles                                  uint          `json:"max-snapshots,omitempty"`
		ExperimentalCompactHashCheckEnabled           bool          `json:"experimental-compact-hash-check-enabled,omitempty"`
		ExperimentalCompactHashCheckTime              time.Duration `json:"experimental-compact-hash-check-time,omitempty"`
		ExperimentalWarningUnaryRequestDuration       time.Duration `json:"experimental-warning-unary-request-duration,omitempty"`
		ExperimentalCorruptCheckTime                  time.Duration `json:"experimental-corrupt-check-time,omitempty"`
		ExperimentalCompactionBatchLimit              int           `json:"experimental-compaction-batch-limit,omitempty"`
		ExperimentalWatchProgressNotifyInterval       time.Duration `json:"experimental-watch-progress-notify-interval,omitempty"`
		ExperimentalWarningApplyDuration              time.Duration `json:"experimental-warning-apply-duration,omitempty"`
		ExperimentalBootstrapDefragThresholdMegabytes uint          `json:"experimental-bootstrap-defrag-threshold-megabytes,omitempty"`
		ExperimentalSnapshotCatchUpEntries            uint64        `json:"experimental-snapshot-catch-up-entries,omitempty"`
		ExperimentalCompactionSleepInterval           time.Duration `json:"experimental-compaction-sleep-interval,omitempty"`
		ExperimentalDowngradeCheckTime                time.Duration `json:"experimental-downgrade-check-time,omitempty"`
	}

	testCases := []struct {
		name           string
		configFileYAML configFileYAML
		expectedFlags  map[string]struct{}
	}{
		{
			name:           "no deprecated options",
			configFileYAML: configFileYAML{},
			expectedFlags:  map[string]struct{}{},
		},
		{
			name: "deprecated experimental options",
			configFileYAML: configFileYAML{
				ExperimentalCompactHashCheckEnabled:           true,
				ExperimentalCompactHashCheckTime:              2 * time.Minute,
				ExperimentalWarningUnaryRequestDuration:       time.Second,
				ExperimentalCorruptCheckTime:                  time.Minute,
				ExperimentalCompactionBatchLimit:              1,
				ExperimentalWatchProgressNotifyInterval:       3 * time.Minute,
				ExperimentalWarningApplyDuration:              3 * time.Minute,
				ExperimentalBootstrapDefragThresholdMegabytes: 100,
				ExperimentalSnapshotCatchUpEntries:            1000,
				ExperimentalCompactionSleepInterval:           30 * time.Second,
				ExperimentalDowngradeCheckTime:                1 * time.Minute,
			},
			expectedFlags: map[string]struct{}{
				"experimental-compact-hash-check-enabled":           {},
				"experimental-compact-hash-check-time":              {},
				"experimental-corrupt-check-time":                   {},
				"experimental-compaction-batch-limit":               {},
				"experimental-watch-progress-notify-interval":       {},
				"experimental-warning-apply-duration":               {},
				"experimental-bootstrap-defrag-threshold-megabytes": {},
				"experimental-snapshot-catchup-entries":             {},
				"experimental-compaction-sleep-interval":            {},
				"experimental-downgrade-check-time":                 {},
			},
		},
		{
			name: "deprecated snapshot options",
			configFileYAML: configFileYAML{
				SnapshotCount: 10000,
				MaxSnapFiles:  5,
			},
			expectedFlags: map[string]struct{}{
				"snapshot-count": {},
				"max-snapshots":  {},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create config file
			b, err := yaml.Marshal(&tc.configFileYAML)
			if err != nil {
				t.Fatal(err)
			}

			tmpfile := mustCreateCfgFile(t, b)
			defer os.Remove(tmpfile.Name())

			// Parse config
			cfg := newConfig()
			err = cfg.parse([]string{fmt.Sprintf("--config-file=%s", tmpfile.Name())})
			if err != nil {
				t.Fatal(err)
			}

			// Check which flags were set and marked as deprecated
			foundFlags := make(map[string]struct{})
			for flagName := range cfg.ec.FlagsExplicitlySet {
				if _, ok := deprecatedFlags[flagName]; ok {
					foundFlags[flagName] = struct{}{}
				}
			}

			// Compare sets of flags
			assert.Equalf(t, tc.expectedFlags, foundFlags, "deprecated flags mismatch - expected: %v, got: %v",
				tc.expectedFlags, foundFlags)

			// Note: experimental-warning-unary-request-duration deprecation is handled
			// through a separate mechanism in embed.Config
			if tc.configFileYAML.ExperimentalWarningUnaryRequestDuration != 0 {
				assert.Equalf(t, cfg.ec.WarningUnaryRequestDuration,
					tc.configFileYAML.ExperimentalWarningUnaryRequestDuration,
					"experimental warning duration mismatch - expected: %v, got: %v",
					tc.configFileYAML.ExperimentalWarningUnaryRequestDuration,
					cfg.ec.WarningUnaryRequestDuration)
			}
		})
	}
}

// TestPeerSkipClientSanVerificationFlagMigration tests the migration from
// --experimental-peer-skip-client-san-verification to --peer-skip-client-san-verification
// TODO: delete in v3.7
func TestPeerSkipClientSanVerificationFlagMigration(t *testing.T) {
	testCases := []struct {
		name                                      string
		peerSkipClientSanVerification             string
		experimentalPeerSkipClientSanVerification string
		useConfigFile                             bool
		expectErr                                 bool
		expectedPeerSkipClientSanVerification     bool
	}{
		{
			name:                          "cannot set both experimental flag and non experimental flag",
			peerSkipClientSanVerification: "true",
			experimentalPeerSkipClientSanVerification: "true",
			expectErr: true,
		},
		{
			name: "can set experimental flag to true",
			experimentalPeerSkipClientSanVerification: "true",
			expectedPeerSkipClientSanVerification:     true,
		},
		{
			name: "can set experimental flag to false",
			experimentalPeerSkipClientSanVerification: "false",
			expectedPeerSkipClientSanVerification:     false,
		},
		{
			name:                                  "can set non experimental flag to true",
			peerSkipClientSanVerification:         "true",
			expectedPeerSkipClientSanVerification: true,
		},
		{
			name:                                  "can set non experimental flag to false",
			peerSkipClientSanVerification:         "false",
			expectedPeerSkipClientSanVerification: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			type securityConfig struct {
				SkipClientSanVerification bool `json:"skip-client-san-verification,omitempty"`
			}
			cmdLineArgs := []string{}
			yc := struct {
				ExperimentalPeerSkipClientSanVerification bool           `json:"experimental-peer-skip-client-san-verification,omitempty"`
				PeerSecurityJSON                          securityConfig `json:"peer-transport-security"`
			}{}

			if tc.peerSkipClientSanVerification != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--peer-skip-client-san-verification=%s", tc.peerSkipClientSanVerification))
				val, err := strconv.ParseBool(tc.peerSkipClientSanVerification)
				if err != nil {
					t.Fatal(err)
				}
				yc.PeerSecurityJSON.SkipClientSanVerification = val
			}

			if tc.experimentalPeerSkipClientSanVerification != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-peer-skip-client-san-verification=%s", tc.experimentalPeerSkipClientSanVerification))
				val, err := strconv.ParseBool(tc.experimentalPeerSkipClientSanVerification)
				if err != nil {
					t.Fatal(err)
				}
				yc.ExperimentalPeerSkipClientSanVerification = val
			}
			cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile := generateCfgsFromFileAndCmdLine(t, yc, cmdLineArgs)

			if tc.expectErr {
				if errFromCmdLine == nil || errFromFile == nil {
					t.Fatalf("expect parse error, got errFromCmdLine=%v, errFromFile=%v", errFromCmdLine, errFromFile)
				}
				return
			}
			if errFromCmdLine != nil || errFromFile != nil {
				t.Fatal("error parsing config")
			}
			if cfgFromCmdLine.ec.PeerTLSInfo.SkipClientSANVerify != tc.expectedPeerSkipClientSanVerification {
				t.Errorf("expected SkipClientSANVerify=%v, got %v",
					tc.expectedPeerSkipClientSanVerification,
					cfgFromCmdLine.ec.PeerTLSInfo.SkipClientSANVerify)
			}
			if cfgFromFile.ec.PeerTLSInfo.SkipClientSANVerify != tc.expectedPeerSkipClientSanVerification {
				t.Errorf("expected SkipClientSANVerify=%v, got %v",
					tc.expectedPeerSkipClientSanVerification,
					cfgFromFile.ec.PeerTLSInfo.SkipClientSANVerify)
			}
		})
	}
}

// TestDistributedTracingFlagsMigration tests the migration from
// --experimental-distributed-tracing-* to --distributed-tracing-*
// TODO: delete in v3.7
func TestDistributedTracingFlagsMigration(t *testing.T) {
	testCases := []struct {
		name string

		enableDistributedTracing                 string
		distributedTracingAddress                string
		distributedTracingServiceName            string
		distributedTracingServiceInstanceID      string
		distributedTracingSamplingRatePerMillion string

		experimentalEnableDistributedTracing                 string
		experimentalDistributedTracingAddress                string
		experimentalDistributedTracingServiceName            string
		experimentalDistributedTracingServiceInstanceID      string
		experimentalDistributedTracingSamplingRatePerMillion string

		expectedEnableDistributedTracing                 bool
		expectedDistributedTracingAddress                string
		expectedDistributedTracingServiceName            string
		expectedDistributedTracingServiceInstanceID      string
		expectedDistributedTracingSamplingRatePerMillion int

		expectErr bool
	}{
		// cannot set both experimental flags and non-experimental flags
		{
			name:                                 "cannot set both experimental flag and non experimental flag 1",
			enableDistributedTracing:             "true",
			experimentalEnableDistributedTracing: "true",
			expectErr:                            true,
		},
		{
			name:                                  "cannot set both experimental flag and non experimental flag 2",
			distributedTracingAddress:             "localhost:4317",
			experimentalDistributedTracingAddress: "localhost:4318",
			expectErr:                             true,
		},
		{
			name:                          "cannot set both experimental flag and non experimental flag 3",
			distributedTracingServiceName: "etcd1",
			experimentalDistributedTracingServiceName: "etcd2",
			expectErr: true,
		},
		{
			name:                                "cannot set both experimental flag and non experimental flag 4",
			distributedTracingServiceInstanceID: "fakeID",
			experimentalDistributedTracingServiceInstanceID: "fakeID",
			expectErr: true,
		},
		{
			name:                                     "cannot set both experimental flag and non experimental flag 5",
			distributedTracingSamplingRatePerMillion: "100",
			experimentalDistributedTracingSamplingRatePerMillion: "100",
			expectErr: true,
		},
		// can set either --experimental-enable-distributed-tracing or --enable-distributed-tracing
		{
			name:                                 "can set experimental-enable-distributed-tracing to true",
			experimentalEnableDistributedTracing: "true",
			expectedEnableDistributedTracing:     true,
		},
		{
			name:                                 "can set experimental-enable-distributed-tracing to false",
			experimentalEnableDistributedTracing: "false",
			expectedEnableDistributedTracing:     false,
		},
		{
			name:                             "can set enable-distributed-tracing to true",
			enableDistributedTracing:         "true",
			expectedEnableDistributedTracing: true,
		},
		{
			name:                             "can set enable-distributed-tracing to false",
			enableDistributedTracing:         "false",
			expectedEnableDistributedTracing: false,
		},
		// can set either --experimental-distributed-tracing-address or --distributed-tracing-address
		{
			name:                                  "can set experimental-distributed-tracing-address",
			experimentalDistributedTracingAddress: "localhost:1234",
			expectedDistributedTracingAddress:     "localhost:1234",
		},
		{
			name:                              "can set distributed-tracing-address",
			distributedTracingAddress:         "localhost:1234",
			expectedDistributedTracingAddress: "localhost:1234",
		},
		// can set either --experimental-distributed-tracing-service-name or --distributed-tracing-service-name
		{
			name: "can set experimental-distributed-tracing-service-name",
			experimentalDistributedTracingServiceName: "fakeSererName",
			expectedDistributedTracingServiceName:     "fakeSererName",
		},
		{
			name:                                  "can set distributed-tracing-service-name",
			distributedTracingServiceName:         "fakeSererName",
			expectedDistributedTracingServiceName: "fakeSererName",
		},
		// can set either --experimental-distributed-tracing-instance-id or --distributed-tracing-instance-id
		{
			name: "can set experimental-distributed-tracing-instance-id",
			experimentalDistributedTracingServiceInstanceID: "fakeID",
			expectedDistributedTracingServiceInstanceID:     "fakeID",
		},
		{
			name:                                "can set distributed-tracing-instance-id",
			distributedTracingServiceInstanceID: "fakeID",
			expectedDistributedTracingServiceInstanceID: "fakeID",
		},
		// can set either --experimental-distributed-tracing-sampling-rate or --distributed-tracing-sampling-rate
		{
			name: "can set experimental-distributed-tracing-sampling-rate",
			experimentalDistributedTracingSamplingRatePerMillion: "200",
			expectedDistributedTracingSamplingRatePerMillion:     200,
		},
		{
			name:                                     "can set distributed-tracing-sampling-rate",
			distributedTracingSamplingRatePerMillion: "300",
			expectedDistributedTracingSamplingRatePerMillion: 300,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdLineArgs := []string{}
			yc := struct {
				ExperimentalEnableDistributedTracing bool `json:"experimental-enable-distributed-tracing,omitempty"`
				EnableDistributedTracing             bool `json:"enable-distributed-tracing,omitempty"`

				ExperimentalDistributedTracingAddress string `json:"experimental-distributed-tracing-address,omitempty"`
				DistributedTracingAddress             string `json:"distributed-tracing-address,omitempty"`

				ExperimentalDistributedTracingServiceName string `json:"experimental-distributed-tracing-service-name,omitempty"`
				DistributedTracingServiceName             string `json:"distributed-tracing-service-name,omitempty"`

				ExperimentalDistributedTracingServiceInstanceID string `json:"experimental-distributed-tracing-instance-id,omitempty"`
				DistributedTracingServiceInstanceID             string `json:"distributed-tracing-instance-id,omitempty"`

				ExperimentalDistributedTracingSamplingRatePerMillion int `json:"experimental-distributed-tracing-sampling-rate,omitempty"`
				DistributedTracingSamplingRatePerMillion             int `json:"distributed-tracing-sampling-rate,omitempty"`
			}{}

			// --enable-distributed-tracing and --experimental-enable-distributed-tracing
			if tc.enableDistributedTracing != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--enable-distributed-tracing=%s", tc.enableDistributedTracing))
				val, err := strconv.ParseBool(tc.enableDistributedTracing)
				require.NoError(t, err)
				yc.EnableDistributedTracing = val
			}
			if tc.experimentalEnableDistributedTracing != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-enable-distributed-tracing=%s", tc.experimentalEnableDistributedTracing))
				val, err := strconv.ParseBool(tc.experimentalEnableDistributedTracing)
				require.NoError(t, err)
				yc.ExperimentalEnableDistributedTracing = val
			}

			// --distributed-tracing-address and --experimental-distributed-tracing-address
			if tc.distributedTracingAddress != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--distributed-tracing-address=%s", tc.distributedTracingAddress))
				yc.DistributedTracingAddress = tc.distributedTracingAddress
			}
			if tc.experimentalDistributedTracingAddress != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-distributed-tracing-address=%s", tc.experimentalDistributedTracingAddress))
				yc.ExperimentalDistributedTracingAddress = tc.experimentalDistributedTracingAddress
			}

			// --distributed-tracing-service-name and --experimental-distributed-tracing-service-name
			if tc.distributedTracingServiceName != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--distributed-tracing-service-name=%s", tc.distributedTracingServiceName))
				yc.DistributedTracingServiceName = tc.distributedTracingServiceName
			}
			if tc.experimentalDistributedTracingServiceName != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-distributed-tracing-service-name=%s", tc.experimentalDistributedTracingServiceName))
				yc.ExperimentalDistributedTracingServiceName = tc.experimentalDistributedTracingServiceName
			}

			// --distributed-tracing-instance-id and --experimental-distributed-tracing-instance-id
			if tc.distributedTracingServiceInstanceID != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--distributed-tracing-instance-id=%s", tc.distributedTracingServiceInstanceID))
				yc.DistributedTracingServiceInstanceID = tc.distributedTracingServiceInstanceID
			}
			if tc.experimentalDistributedTracingServiceInstanceID != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-distributed-tracing-instance-id=%s", tc.experimentalDistributedTracingServiceInstanceID))
				yc.ExperimentalDistributedTracingServiceInstanceID = tc.experimentalDistributedTracingServiceInstanceID
			}

			// --distributed-tracing-sampling-rate and --experimental-distributed-tracing-sampling-rate
			if tc.distributedTracingSamplingRatePerMillion != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--distributed-tracing-sampling-rate=%s", tc.distributedTracingSamplingRatePerMillion))
				val, err := strconv.ParseInt(tc.distributedTracingSamplingRatePerMillion, 10, 64)
				require.NoError(t, err)
				yc.DistributedTracingSamplingRatePerMillion = int(val)
			}
			if tc.experimentalDistributedTracingSamplingRatePerMillion != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-distributed-tracing-sampling-rate=%s", tc.experimentalDistributedTracingSamplingRatePerMillion))
				val, err := strconv.ParseInt(tc.experimentalDistributedTracingSamplingRatePerMillion, 10, 64)
				require.NoError(t, err)
				yc.ExperimentalDistributedTracingSamplingRatePerMillion = int(val)
			}

			cfgFromCmdLine, errFromCmdLine, cfgFromFile, errFromFile := generateCfgsFromFileAndCmdLine(t, yc, cmdLineArgs)

			if tc.expectErr {
				if errFromCmdLine == nil || errFromFile == nil {
					t.Fatalf("expect parse error, got errFromCmdLine=%v, errFromFile=%v", errFromCmdLine, errFromFile)
				}
				return
			}
			if errFromCmdLine != nil || errFromFile != nil {
				t.Fatal("error parsing config")
			}

			// verify the expected values
			require.Equal(t, tc.expectedEnableDistributedTracing, cfgFromCmdLine.ec.EnableDistributedTracing)
			require.Equal(t, tc.expectedEnableDistributedTracing, cfgFromFile.ec.EnableDistributedTracing)

			if tc.expectedDistributedTracingAddress != "" {
				require.Equal(t, tc.expectedDistributedTracingAddress, cfgFromCmdLine.ec.DistributedTracingAddress)
				require.Equal(t, tc.expectedDistributedTracingAddress, cfgFromFile.ec.DistributedTracingAddress)
			}

			if tc.expectedDistributedTracingServiceName != "" {
				require.Equal(t, tc.expectedDistributedTracingServiceName, cfgFromCmdLine.ec.DistributedTracingServiceName)
				require.Equal(t, tc.expectedDistributedTracingServiceName, cfgFromFile.ec.DistributedTracingServiceName)
			}

			if tc.expectedDistributedTracingServiceInstanceID != "" {
				require.Equal(t, tc.expectedDistributedTracingServiceInstanceID, cfgFromCmdLine.ec.DistributedTracingServiceInstanceID)
				require.Equal(t, tc.expectedDistributedTracingServiceInstanceID, cfgFromFile.ec.DistributedTracingServiceInstanceID)
			}

			require.Equal(t, tc.expectedDistributedTracingSamplingRatePerMillion, cfgFromCmdLine.ec.DistributedTracingSamplingRatePerMillion)
			require.Equal(t, tc.expectedDistributedTracingSamplingRatePerMillion, cfgFromFile.ec.DistributedTracingSamplingRatePerMillion)
		})
	}
}
