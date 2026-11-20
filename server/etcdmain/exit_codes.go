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

package etcdmain

// Exit codes used by the etcd server binary.
//
// These exit codes follow the convention established by
// https://tldp.org/LDP/abs/html/exitcodes.html and extend it with
// codes specific to etcd. Codes 0-2 are reserved for standard meanings.
// Codes >= 128 indicate exit by signal (signal number = code - 128).
//
// Exit code classification:
//
//	0  - ExitSuccess          Normal shutdown (signal handling, graceful stop, or successful --help/--version)
//	1  - ExitError            General failure (configuration error, startup failure, unsupported architecture, discovery failure)
//	2  - ExitBadArgs          Command-line argument parsing failure (invalid flag values)
//
// The following table lists every os.Exit call site and its exit code.
//
//	Exit code: 0 (ExitSuccess)
//	  - config.go:115   - --help flag was requested
//	  - config.go:128   - --version flag was requested
//	  - etcd.go:176     - Normal shutdown after stopped channel closes (osutil.Exit(0))
//	  - interrupt_unix.go:74 - init process (pid 1) received SIGINT/SIGTERM, cannot re-raise
//
//	Exit code: 1 (ExitError)
//	  - etcd.go:60      - Failed to create default zap logger before config parsing
//	  - etcd.go:69      - Configuration validation failed after parsing flags
//	  - etcd.go:141     - Discovery bootstrap failed (token already used)
//	  - etcd.go:155     - Initial cluster misconfiguration (missing --initial-cluster etc.)
//	  - etcd.go:252     - Refusing to run on unsupported architecture without ETCD_UNSUPPORTED_ARCH env
//	  - main.go:34      - gateway/grpc-proxy subcommand execution failed
//	  - util.go:34      - DNS SRV record discovery failed for bootstrap endpoints
//	  - embed/config.go:811  - Invalid listen-peer-urls in config file
//	  - embed/config.go:820  - Invalid listen-client-urls in config file
//	  - embed/config.go:829  - Invalid listen-client-http-urls in config file
//	  - embed/config.go:838  - Invalid initial-advertise-peer-urls in config file
//	  - embed/config.go:847  - Invalid advertise-client-urls in config file
//	  - embed/config.go:856  - Invalid listen-metrics-urls in config file
//	  - gateway.go:*    - Various gateway initialization and runtime failures
//	  - grpc_proxy.go:* - Various grpc-proxy initialization and runtime failures
//
//	Exit code: 2 (ExitBadArgs)
//	  - config.go:117   - Flag parsing error (invalid flag name or value)
const (
	// ExitSuccess indicates the process completed without error.
	// This covers both explicit success paths (--help, --version) and
	// normal shutdown via signal handling or server stop.
	ExitSuccess = 0

	// ExitError is a catch-all code for runtime errors during startup
	// or operation. The specific cause can be determined from the
	// preceding log message.
	//
	// Common sub-categories:
	//   - Configuration validation errors
	//   - Discovery/bootstrap failures
	//   - Architecture not supported
	//   - Listener errors
	//   - Gateway/grpc-proxy initialization failures
	ExitError = 1

	// ExitBadArgs indicates the command line could not be parsed.
	// This differs from ExitError in that it specifically means
	// an invalid flag was provided.
	ExitBadArgs = 2
)
