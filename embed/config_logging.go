// Copyright 2018 The etcd Authors
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

package embed

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"sync"

	"go.etcd.io/etcd/pkg/logutil"

	"github.com/coreos/pkg/capnslog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

// GetLogger returns the logger.
func (cfg Config) GetLogger() *zap.Logger {
	cfg.loggerMu.RLock()
	l := cfg.logger
	cfg.loggerMu.RUnlock()
	return l
}

// for testing
var grpcLogOnce = new(sync.Once)

// setupLogging initializes etcd logging.
// Must be called after flag parsing or finishing configuring embed.Config.
func (cfg *Config) setupLogging() error {
	// handle "DeprecatedLogOutput" in v3.4
	// TODO: remove "DeprecatedLogOutput" in v3.5
	len1 := len(cfg.DeprecatedLogOutput)
	len2 := len(cfg.LogOutputs)
	if len1 != len2 {
		switch {
		case len1 > len2: // deprecate "log-output" flag is used
			fmt.Fprintln(os.Stderr, "'--log-output' flag has been deprecated! Please use '--log-outputs'!")
			cfg.LogOutputs = cfg.DeprecatedLogOutput
		case len1 < len2: // "--log-outputs" flag has been set with multiple writers
			cfg.DeprecatedLogOutput = []string{}
		}
	} else {
		if len1 > 1 {
			return errors.New("both '--log-output' and '--log-outputs' are set; only set '--log-outputs'")
		}
		if len1 < 1 {
			return errors.New("either '--log-output' or '--log-outputs' flag must be set")
		}
		if reflect.DeepEqual(cfg.DeprecatedLogOutput, cfg.LogOutputs) && cfg.DeprecatedLogOutput[0] != DefaultLogOutput {
			return fmt.Errorf("'--log-output=%q' and '--log-outputs=%q' are incompatible; only set --log-outputs", cfg.DeprecatedLogOutput, cfg.LogOutputs)
		}
		if !reflect.DeepEqual(cfg.DeprecatedLogOutput, []string{DefaultLogOutput}) {
			fmt.Fprintf(os.Stderr, "Deprecated '--log-output' flag is set to %q\n", cfg.DeprecatedLogOutput)
			fmt.Fprintln(os.Stderr, "Please use '--log-outputs' flag")
		}
	}

	switch cfg.Logger {
	case "capnslog": // TODO: deprecate this in v3.5
		cfg.ClientTLSInfo.HandshakeFailure = logTLSHandshakeFailure
		cfg.PeerTLSInfo.HandshakeFailure = logTLSHandshakeFailure

		if cfg.Debug {
			capnslog.SetGlobalLogLevel(capnslog.DEBUG)
			grpc.EnableTracing = true
			// enable info, warning, error
			grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stderr, os.Stderr, os.Stderr))
		} else {
			capnslog.SetGlobalLogLevel(capnslog.INFO)
			// only discard info
			grpclog.SetLoggerV2(grpclog.NewLoggerV2(ioutil.Discard, os.Stderr, os.Stderr))
		}

		// TODO: deprecate with "capnslog"
		if cfg.LogPkgLevels != "" {
			repoLog := capnslog.MustRepoLogger("go.etcd.io/etcd")
			settings, err := repoLog.ParseLogLevelConfig(cfg.LogPkgLevels)
			if err != nil {
				plog.Warningf("couldn't parse log level string: %s, continuing with default levels", err.Error())
				return nil
			}
			repoLog.SetLogLevel(settings)
		}

		if len(cfg.LogOutputs) != 1 {
			fmt.Printf("--logger=capnslog supports only 1 value in '--log-outputs', got %q\n", cfg.LogOutputs)
			os.Exit(1)
		}
		// capnslog initially SetFormatter(NewDefaultFormatter(os.Stderr))
		// where NewDefaultFormatter returns NewJournaldFormatter when syscall.Getppid() == 1
		// specify 'stdout' or 'stderr' to skip journald logging even when running under systemd
		output := cfg.LogOutputs[0]
		switch output {
		case StdErrLogOutput:
			capnslog.SetFormatter(capnslog.NewPrettyFormatter(os.Stderr, cfg.Debug))
		case StdOutLogOutput:
			capnslog.SetFormatter(capnslog.NewPrettyFormatter(os.Stdout, cfg.Debug))
		case DefaultLogOutput:
		default:
			plog.Panicf(`unknown log-output %q (only supports %q, %q, %q)`, output, DefaultLogOutput, StdErrLogOutput, StdOutLogOutput)
		}

	case "zap":
		if len(cfg.LogOutputs) == 0 {
			cfg.LogOutputs = []string{DefaultLogOutput}
		}
		if len(cfg.LogOutputs) > 1 {
			for _, v := range cfg.LogOutputs {
				if v == DefaultLogOutput {
					panic(fmt.Errorf("multi logoutput for %q is not supported yet", DefaultLogOutput))
				}
			}
		}

		// TODO: use zapcore to support more features?
		lcfg := zap.Config{
			Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
			Development: false,
			Sampling: &zap.SamplingConfig{
				Initial:    100,
				Thereafter: 100,
			},
			Encoding:      "json",
			EncoderConfig: zap.NewProductionEncoderConfig(),

			OutputPaths:      make([]string, 0),
			ErrorOutputPaths: make([]string, 0),
		}

		outputPaths, errOutputPaths := make(map[string]struct{}), make(map[string]struct{})
		isJournal := false
		for _, v := range cfg.LogOutputs {
			switch v {
			case DefaultLogOutput:
				return errors.New("'--log-outputs=default' is not supported for v3.4 during zap logger migraion (use 'journal', 'stderr', 'stdout', etc.)")

			case JournalLogOutput:
				isJournal = true

			case StdErrLogOutput:
				outputPaths[StdErrLogOutput] = struct{}{}
				errOutputPaths[StdErrLogOutput] = struct{}{}

			case StdOutLogOutput:
				outputPaths[StdOutLogOutput] = struct{}{}
				errOutputPaths[StdOutLogOutput] = struct{}{}

			default:
				outputPaths[v] = struct{}{}
				errOutputPaths[v] = struct{}{}
			}
		}

		if !isJournal {
			for v := range outputPaths {
				lcfg.OutputPaths = append(lcfg.OutputPaths, v)
			}
			for v := range errOutputPaths {
				lcfg.ErrorOutputPaths = append(lcfg.ErrorOutputPaths, v)
			}
			sort.Strings(lcfg.OutputPaths)
			sort.Strings(lcfg.ErrorOutputPaths)

			if cfg.Debug {
				lcfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
				grpc.EnableTracing = true
			}

			var err error
			cfg.logger, err = lcfg.Build()
			if err != nil {
				return err
			}

			cfg.loggerConfig = &lcfg
			cfg.loggerCore = nil
			cfg.loggerWriteSyncer = nil

			grpcLogOnce.Do(func() {
				// debug true, enable info, warning, error
				// debug false, only discard info
				var gl grpclog.LoggerV2
				gl, err = logutil.NewGRPCLoggerV2(lcfg)
				if err == nil {
					grpclog.SetLoggerV2(gl)
				}
			})
			if err != nil {
				return err
			}
		} else {
			if len(cfg.LogOutputs) > 1 {
				for _, v := range cfg.LogOutputs {
					if v != DefaultLogOutput {
						return fmt.Errorf("running with systemd/journal but other '--log-outputs' values (%q) are configured with 'default'; override 'default' value with something else", cfg.LogOutputs)
					}
				}
			}

			// use stderr as fallback
			syncer, lerr := getJournalWriteSyncer()
			if lerr != nil {
				return lerr
			}

			lvl := zap.NewAtomicLevelAt(zap.InfoLevel)
			if cfg.Debug {
				lvl = zap.NewAtomicLevelAt(zap.DebugLevel)
				grpc.EnableTracing = true
			}

			// WARN: do not change field names in encoder config
			// journald logging writer assumes field names of "level" and "caller"
			cr := zapcore.NewCore(
				zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
				syncer,
				lvl,
			)
			cfg.logger = zap.New(cr, zap.AddCaller(), zap.ErrorOutput(syncer))

			cfg.loggerConfig = nil
			cfg.loggerCore = cr
			cfg.loggerWriteSyncer = syncer

			grpcLogOnce.Do(func() {
				grpclog.SetLoggerV2(logutil.NewGRPCLoggerV2FromZapCore(cr, syncer))
			})
		}

		logTLSHandshakeFailure := func(conn *tls.Conn, err error) {
			state := conn.ConnectionState()
			remoteAddr := conn.RemoteAddr().String()
			serverName := state.ServerName
			if len(state.PeerCertificates) > 0 {
				cert := state.PeerCertificates[0]
				ips := make([]string, 0, len(cert.IPAddresses))
				for i := range cert.IPAddresses {
					ips[i] = cert.IPAddresses[i].String()
				}
				cfg.logger.Warn(
					"rejected connection",
					zap.String("remote-addr", remoteAddr),
					zap.String("server-name", serverName),
					zap.Strings("ip-addresses", ips),
					zap.Strings("dns-names", cert.DNSNames),
					zap.Error(err),
				)
			} else {
				cfg.logger.Warn(
					"rejected connection",
					zap.String("remote-addr", remoteAddr),
					zap.String("server-name", serverName),
					zap.Error(err),
				)
			}
		}
		cfg.ClientTLSInfo.HandshakeFailure = logTLSHandshakeFailure
		cfg.PeerTLSInfo.HandshakeFailure = logTLSHandshakeFailure

	default:
		return fmt.Errorf("unknown logger option %q", cfg.Logger)
	}

	return nil
}
