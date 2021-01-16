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
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"go.etcd.io/etcd/pkg/v3/logutil"

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

func setupGrpcLogging(debug bool, config zap.Config) {
	grpcLogOnce.Do(func() {
		// debug true, enable info, warning, error
		// debug false, only discard info
		if debug {
			var gl grpclog.LoggerV2
			gl, err := logutil.NewGRPCLoggerV2(config)
			if err == nil {
				grpclog.SetLoggerV2(gl)
			}
		} else {
			grpclog.SetLoggerV2(grpclog.NewLoggerV2(ioutil.Discard, os.Stderr, os.Stderr))
		}
	})
}

func SetupGrpcLoggingForTest(debug bool) {
	setupGrpcLogging(debug, zap.NewDevelopmentConfig())
}

// setupLogging initializes etcd logging.
// Must be called after flag parsing or finishing configuring embed.Config.
func (cfg *Config) setupLogging() error {
	switch cfg.Logger {
	case "capnslog": // removed in v3.5
		return fmt.Errorf("--logger=capnslog is removed in v3.5")

	case "zap":
		if len(cfg.LogOutputs) == 0 {
			cfg.LogOutputs = []string{DefaultLogOutput}
		}
		if len(cfg.LogOutputs) > 1 {
			for _, v := range cfg.LogOutputs {
				if v == DefaultLogOutput {
					return fmt.Errorf("multi logoutput for %q is not supported yet", DefaultLogOutput)
				}
			}
		}

		outputPaths, errOutputPaths := make([]string, 0), make([]string, 0)
		isJournal := false
		for _, v := range cfg.LogOutputs {
			switch v {
			case DefaultLogOutput:
				outputPaths = append(outputPaths, StdErrLogOutput)
				errOutputPaths = append(errOutputPaths, StdErrLogOutput)

			case JournalLogOutput:
				isJournal = true

			case StdErrLogOutput:
				outputPaths = append(outputPaths, StdErrLogOutput)
				errOutputPaths = append(errOutputPaths, StdErrLogOutput)

			case StdOutLogOutput:
				outputPaths = append(outputPaths, StdOutLogOutput)
				errOutputPaths = append(errOutputPaths, StdOutLogOutput)

			default:
				outputPaths = append(outputPaths, v)
				errOutputPaths = append(errOutputPaths, v)
			}
		}

		if !isJournal {
			copied := logutil.DefaultZapLoggerConfig
			copied.OutputPaths = outputPaths
			copied.ErrorOutputPaths = errOutputPaths
			copied = logutil.MergeOutputPaths(copied)
			copied.Level = zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(cfg.LogLevel))
			if cfg.LogLevel == "debug" {
				grpc.EnableTracing = true
			}
			if cfg.ZapLoggerBuilder == nil {
				cfg.ZapLoggerBuilder = func(c *Config) error {
					var err error
					c.logger, err = copied.Build()
					if err != nil {
						return err
					}
					zap.ReplaceGlobals(c.logger)
					c.loggerMu.Lock()
					defer c.loggerMu.Unlock()
					c.loggerConfig = &copied
					c.loggerCore = nil
					c.loggerWriteSyncer = nil
					setupGrpcLogging(cfg.LogLevel == "debug", copied)
					return nil
				}
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

			lvl := zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(cfg.LogLevel))
			if cfg.LogLevel == "debug" {
				grpc.EnableTracing = true
			}

			// WARN: do not change field names in encoder config
			// journald logging writer assumes field names of "level" and "caller"
			cr := zapcore.NewCore(
				zapcore.NewJSONEncoder(logutil.DefaultZapLoggerConfig.EncoderConfig),
				syncer,
				lvl,
			)
			if cfg.ZapLoggerBuilder == nil {
				cfg.ZapLoggerBuilder = func(c *Config) error {
					c.logger = zap.New(cr, zap.AddCaller(), zap.ErrorOutput(syncer))
					zap.ReplaceGlobals(c.logger)
					c.loggerMu.Lock()
					defer c.loggerMu.Unlock()
					c.loggerConfig = nil
					c.loggerCore = cr
					c.loggerWriteSyncer = syncer

					grpcLogOnce.Do(func() {
						if cfg.LogLevel == "debug" {
							grpclog.SetLoggerV2(logutil.NewGRPCLoggerV2FromZapCore(cr, syncer))
						} else {
							grpclog.SetLoggerV2(grpclog.NewLoggerV2(ioutil.Discard, os.Stderr, os.Stderr))
						}
					})
					return nil
				}
			}
		}

		err := cfg.ZapLoggerBuilder(cfg)
		if err != nil {
			return err
		}

		logTLSHandshakeFailure := func(conn *tls.Conn, err error) {
			state := conn.ConnectionState()
			remoteAddr := conn.RemoteAddr().String()
			serverName := state.ServerName
			if len(state.PeerCertificates) > 0 {
				cert := state.PeerCertificates[0]
				ips := make([]string, len(cert.IPAddresses))
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

// NewZapCoreLoggerBuilder generates a zap core logger builder.
func NewZapCoreLoggerBuilder(lg *zap.Logger, cr zapcore.Core, syncer zapcore.WriteSyncer) func(*Config) error {
	return func(cfg *Config) error {
		cfg.loggerMu.Lock()
		defer cfg.loggerMu.Unlock()
		cfg.logger = lg
		cfg.loggerConfig = nil
		cfg.loggerCore = cr
		cfg.loggerWriteSyncer = syncer

		grpcLogOnce.Do(func() {
			grpclog.SetLoggerV2(logutil.NewGRPCLoggerV2FromZapCore(cr, syncer))
		})
		return nil
	}
}
