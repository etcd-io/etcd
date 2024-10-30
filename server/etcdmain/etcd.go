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
	errorspkg "errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/client/pkg/v3/logutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/osutil"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2discovery"
	"go.etcd.io/etcd/server/v3/etcdserver/errors"
)

type dirType string

var (
	dirMember = dirType("member")
	dirProxy  = dirType("proxy")
	dirEmpty  = dirType("empty")
)

func startEtcdOrProxyV2(args []string) {
	grpc.EnableTracing = false

	cfg := newConfig()
	defaultInitialCluster := cfg.ec.InitialCluster

	err := cfg.parse(args[1:])
	lg := cfg.ec.GetLogger()
	// If we failed to parse the whole configuration, print the error using
	// preferably the resolved logger from the config,
	// but if does not exists, create a new temporary logger.
	if lg == nil {
		var zapError error
		// use this logger
		lg, zapError = logutil.CreateDefaultZapLogger(zap.InfoLevel)
		if zapError != nil {
			fmt.Printf("error creating zap logger %v", zapError)
			os.Exit(1)
		}
	}
	lg.Info("Running: ", zap.Strings("args", args))
	if err != nil {
		lg.Warn("failed to verify flags", zap.Error(err))
		switch {
		case errorspkg.Is(err, embed.ErrUnsetAdvertiseClientURLsFlag):
			lg.Warn("advertise client URLs are not set", zap.Error(err))
		}
		os.Exit(1)
	}

	cfg.ec.SetupGlobalLoggers()

	defer func() {
		logger := cfg.ec.GetLogger()
		if logger != nil {
			logger.Sync()
		}
	}()

	defaultHost, dhErr := (&cfg.ec).UpdateDefaultClusterFromName(defaultInitialCluster)
	if defaultHost != "" {
		lg.Info(
			"detected default host for advertise",
			zap.String("host", defaultHost),
		)
	}
	if dhErr != nil {
		lg.Info("failed to detect default host", zap.Error(dhErr))
	}

	if cfg.ec.Dir == "" {
		cfg.ec.Dir = fmt.Sprintf("%v.etcd", cfg.ec.Name)
		lg.Warn(
			"'data-dir' was empty; using default",
			zap.String("data-dir", cfg.ec.Dir),
		)
	}

	var stopped <-chan struct{}
	var errc <-chan error

	which := identifyDataDirOrDie(cfg.ec.GetLogger(), cfg.ec.Dir)
	if which != dirEmpty {
		lg.Info(
			"server has already been initialized",
			zap.String("data-dir", cfg.ec.Dir),
			zap.String("dir-type", string(which)),
		)
		switch which {
		case dirMember:
			stopped, errc, err = startEtcd(&cfg.ec)
		case dirProxy:
			lg.Panic("v2 http proxy has already been deprecated in 3.6", zap.String("dir-type", string(which)))
		default:
			lg.Panic(
				"unknown directory type",
				zap.String("dir-type", string(which)),
			)
		}
	} else {
		lg.Info(
			"Initialize and start etcd server",
			zap.String("data-dir", cfg.ec.Dir),
			zap.String("dir-type", string(which)),
		)
		stopped, errc, err = startEtcd(&cfg.ec)
	}

	if err != nil {
		var derr *errors.DiscoveryError
		if errorspkg.As(err, &derr) {
			switch {
			case errorspkg.Is(derr.Err, v2discovery.ErrDuplicateID):
				lg.Warn(
					"member has been registered with discovery service",
					zap.String("name", cfg.ec.Name),
					zap.String("discovery-token", cfg.ec.Durl),
					zap.Error(derr.Err),
				)
				lg.Warn(
					"but could not find valid cluster configuration",
					zap.String("data-dir", cfg.ec.Dir),
				)
				lg.Warn("check data dir if previous bootstrap succeeded")
				lg.Warn("or use a new discovery token if previous bootstrap failed")

			case errorspkg.Is(derr.Err, v2discovery.ErrDuplicateName):
				lg.Warn(
					"member with duplicated name has already been registered",
					zap.String("discovery-token", cfg.ec.Durl),
					zap.Error(derr.Err),
				)
				lg.Warn("cURL the discovery token URL for details")
				lg.Warn("do not reuse discovery token; generate a new one to bootstrap a cluster")

			default:
				lg.Warn(
					"failed to bootstrap; discovery token was already used",
					zap.String("discovery-token", cfg.ec.Durl),
					zap.Error(err),
				)
				lg.Warn("do not reuse discovery token; generate a new one to bootstrap a cluster")
			}
			os.Exit(1)
		}

		if strings.Contains(err.Error(), "include") && strings.Contains(err.Error(), "--initial-cluster") {
			lg.Warn("failed to start", zap.Error(err))
			if cfg.ec.InitialCluster == cfg.ec.InitialClusterFromName(cfg.ec.Name) {
				lg.Warn("forgot to set --initial-cluster?")
			}
			if types.URLs(cfg.ec.AdvertisePeerUrls).String() == embed.DefaultInitialAdvertisePeerURLs {
				lg.Warn("forgot to set --initial-advertise-peer-urls?")
			}
			if cfg.ec.InitialCluster == cfg.ec.InitialClusterFromName(cfg.ec.Name) && len(cfg.ec.Durl) == 0 && len(cfg.ec.DiscoveryCfg.Endpoints) == 0 {
				lg.Warn("V2 discovery settings (i.e., --discovery) or v3 discovery settings (i.e., --discovery-token, --discovery-endpoints) are not set")
			}
			os.Exit(1)
		}
		lg.Fatal("discovery failed", zap.Error(err))
	}

	osutil.HandleInterrupts(lg)

	// At this point, the initialization of etcd is done.
	// The listeners are listening on the TCP ports and ready
	// for accepting connections. The etcd instance should be
	// joined with the cluster and ready to serve incoming
	// connections.
	notifySystemd(lg)

	select {
	case lerr := <-errc:
		// fatal out on listener errors
		lg.Fatal("listener failed", zap.Error(lerr))
	case <-stopped:
	}

	osutil.Exit(0)
}

// startEtcd runs StartEtcd in addition to hooks needed for standalone etcd.
func startEtcd(cfg *embed.Config) (<-chan struct{}, <-chan error, error) {
	e, err := embed.StartEtcd(cfg)
	if err != nil {
		return nil, nil, err
	}
	osutil.RegisterInterruptHandler(e.Close)
	select {
	case <-e.Server.ReadyNotify(): // wait for e.Server to join the cluster
	case <-e.Server.StopNotify(): // publish aborted from 'ErrStopped'
	}
	return e.Server.StopNotify(), e.Err(), nil
}

// identifyDataDirOrDie returns the type of the data dir.
// Dies if the datadir is invalid.
func identifyDataDirOrDie(lg *zap.Logger, dir string) dirType {
	names, err := fileutil.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return dirEmpty
		}
		lg.Fatal("failed to list data directory", zap.String("dir", dir), zap.Error(err))
	}

	var m, p bool
	for _, name := range names {
		switch dirType(name) {
		case dirMember:
			m = true
		case dirProxy:
			p = true
		default:
			lg.Warn(
				"found invalid file under data directory",
				zap.String("filename", name),
				zap.String("data-dir", dir),
			)
		}
	}

	if m && p {
		lg.Fatal("invalid datadir; both member and proxy directories exist")
	}
	if m {
		return dirMember
	}
	if p {
		return dirProxy
	}
	return dirEmpty
}

func checkSupportArch() {
	lg, err := logutil.CreateDefaultZapLogger(zap.InfoLevel)
	if err != nil {
		panic(err)
	}
	// To add a new platform, check https://github.com/etcd-io/website/blob/main/content/en/docs/${VERSION}/op-guide/supported-platform.md.
	// The ${VERSION} is the etcd version, e.g. v3.5, v3.6 etc.
	switch runtime.GOARCH {
	case "amd64", "arm64", "ppc64le", "s390x":
		return
	}
	// unsupported arch only configured via environment variable
	// so unset here to not parse through flag
	defer os.Unsetenv("ETCD_UNSUPPORTED_ARCH")
	if env, ok := os.LookupEnv("ETCD_UNSUPPORTED_ARCH"); ok && env == runtime.GOARCH {
		lg.Info("running etcd on unsupported architecture since ETCD_UNSUPPORTED_ARCH is set", zap.String("arch", env))
		return
	}

	lg.Error("Refusing to run etcd on unsupported architecture since ETCD_UNSUPPORTED_ARCH is not set", zap.String("arch", runtime.GOARCH))
	os.Exit(1)
}
