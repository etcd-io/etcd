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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"

	"go.etcd.io/etcd/pkg/v3/featuregate"
	"go.etcd.io/etcd/pkg/v3/flags"
	"go.etcd.io/etcd/server/v3/embed"
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
		SnapshotCatchUpEntries uint64 `json:"experimental-snapshot-catch-up-entries"`
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
		if err := cfg.parse(args); err != nil {
			t.Fatalf("#%d: unexpected clusterState.Set error: %v", i, err)
		}
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
				features.DistributedTracing:      false,
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
				"--feature-gates=DistributedTracing=true",
			},
			expectedFeatures: map[featuregate.Feature]bool{
				features.StopGRPCServiceOnDefrag: true,
				features.DistributedTracing:      true,
			},
		},
		{
			name: "can set feature gate from experimental flag",
			args: []string{
				"--experimental-stop-grpc-service-on-defrag=true",
			},
			expectedFeatures: map[featuregate.Feature]bool{
				features.StopGRPCServiceOnDefrag: true,
				features.DistributedTracing:      false,
			},
		},
		{
			name: "can set feature gate from feature gate flag",
			args: []string{
				"--feature-gates=StopGRPCServiceOnDefrag=true,DistributedTracing=true",
			},
			expectedFeatures: map[featuregate.Feature]bool{
				features.StopGRPCServiceOnDefrag: true,
				features.DistributedTracing:      true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newConfig()
			err := cfg.parse(tc.args)
			if tc.expectErr {
				if err == nil {
					t.Fatal("expect parse error")
				}
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
				if err != nil {
					t.Fatal(err)
				}
				yc.CompactHashCheckTime = compactHashCheckTime
			}

			if tc.experimentalCompactHashCheckTime != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-compact-hash-check-time=%s", tc.experimentalCompactHashCheckTime))
				experimentalCompactHashCheckTime, err := time.ParseDuration(tc.experimentalCompactHashCheckTime)
				if err != nil {
					t.Fatal(err)
				}
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

			if cfgFromCmdLine.ec.CompactHashCheckTime != tc.expectedCompactHashCheckTime {
				t.Errorf("expected CompactHashCheckTime=%v, got %v", tc.expectedCompactHashCheckTime, cfgFromCmdLine.ec.CompactHashCheckTime)
			}
			if cfgFromFile.ec.CompactHashCheckTime != tc.expectedCompactHashCheckTime {
				t.Errorf("expected CompactHashCheckTime=%v, got %v", tc.expectedCompactHashCheckTime, cfgFromFile.ec.CompactHashCheckTime)
			}
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
				if err != nil {
					t.Fatal(err)
				}
				yc.CorruptCheckTime = corruptCheckTime
			}

			if tc.experimentalCorruptCheckTime != "" {
				cmdLineArgs = append(cmdLineArgs, fmt.Sprintf("--experimental-corrupt-check-time=%s", tc.experimentalCorruptCheckTime))
				experimentalCorruptCheckTime, err := time.ParseDuration(tc.experimentalCorruptCheckTime)
				if err != nil {
					t.Fatal(err)
				}
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

			if cfgFromCmdLine.ec.CorruptCheckTime != tc.expectedCorruptCheckTime {
				t.Errorf("expected CorruptCheckTime=%v, got %v", tc.expectedCorruptCheckTime, cfgFromCmdLine.ec.CorruptCheckTime)
			}
			if cfgFromFile.ec.CorruptCheckTime != tc.expectedCorruptCheckTime {
				t.Errorf("expected CorruptCheckTime=%v, got %v", tc.expectedCorruptCheckTime, cfgFromFile.ec.CorruptCheckTime)
			}
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

			if cfgFromCmdLine.ec.CompactionBatchLimit != tc.expectedCompactionBatchLimit {
				t.Errorf("expected CorruptCheckTime=%v, got %v", tc.expectedCompactionBatchLimit, cfgFromCmdLine.ec.CompactionBatchLimit)
			}
			if cfgFromFile.ec.CompactionBatchLimit != tc.expectedCompactionBatchLimit {
				t.Errorf("expected CorruptCheckTime=%v, got %v", tc.expectedCompactionBatchLimit, cfgFromFile.ec.CompactionBatchLimit)
			}
		})
	}
}

// TODO delete in v3.7
func generateCfgsFromFileAndCmdLine(t *testing.T, yc any, cmdLineArgs []string) (*config, error, *config, error) {
	b, err := yaml.Marshal(&yc)
	if err != nil {
		t.Fatal(err)
	}

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
		SnapshotCount                           uint64        `json:"snapshot-count,omitempty"`
		MaxSnapFiles                            uint          `json:"max-snapshots,omitempty"`
		ExperimentalCompactHashCheckEnabled     bool          `json:"experimental-compact-hash-check-enabled,omitempty"`
		ExperimentalCompactHashCheckTime        time.Duration `json:"experimental-compact-hash-check-time,omitempty"`
		ExperimentalWarningUnaryRequestDuration time.Duration `json:"experimental-warning-unary-request-duration,omitempty"`
		ExperimentalCorruptCheckTime            time.Duration `json:"experimental-corrupt-check-time,omitempty"`
		ExperimentalCompactionBatchLimit        int           `json:"experimental-compaction-batch-limit,omitempty"`
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
				ExperimentalCompactHashCheckEnabled:     true,
				ExperimentalCompactHashCheckTime:        2 * time.Minute,
				ExperimentalWarningUnaryRequestDuration: time.Second,
				ExperimentalCorruptCheckTime:            time.Minute,
				ExperimentalCompactionBatchLimit:        1,
			},
			expectedFlags: map[string]struct{}{
				"experimental-compact-hash-check-enabled": {},
				"experimental-compact-hash-check-time":    {},
				"experimental-corrupt-check-time":         {},
				"experimental-compaction-batch-limit":     {},
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
