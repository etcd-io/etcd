# Exit Codes Reference

This document provides a reference of exit codes returned by the etcd server.

etcd server explicitly uses three exit codes: **0** (success), **1** (general errors), and **2** (argument errors). When terminated by signals (SIGTERM/SIGINT) on Linux/Unix systems, the exit code depends on the process type: PID 1 processes exit with code 0, while non-PID 1 processes re-raise the signal, resulting in exit codes 143 (SIGTERM) or 130 (SIGINT).

## Exit Code 0 - Success

| Scenario | Code Reference |
| -------- | -------------- |
| Help flag (`--help`) | [`server/etcdmain/config.go#L131`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/config.go#L131) |
| Version flag (`--version`) | [`server/etcdmain/config.go#L144`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/config.go#L144) |
| Normal shutdown | [`server/etcdmain/etcd.go#L176`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/etcd.go#L176) |
| Graceful shutdown on signal (PID 1) | [`pkg/osutil/interrupt_unix.go#L74`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/pkg/osutil/interrupt_unix.go#L74) |

## Signal Termination (128 + signal number)

**Note:** This behavior is specific to Linux platform. On other platforms, etcd typically returns exit code 0 if it exits without error, and 1 otherwise.

For non-PID 1 processes on Linux/Unix, the signal handler re-raises the signal to the process itself ([`pkg/osutil/interrupt_unix.go#L77`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/pkg/osutil/interrupt_unix.go#L77)), which results in the kernel setting the exit code to 128 + signal number.

| Signal | Exit Code | Code Reference |
| ------ | --------- | -------------- |
| SIGINT (Ctrl-C) | 130 (128 + 2) | [`pkg/osutil/interrupt_unix.go#L53`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/pkg/osutil/interrupt_unix.go#L53) |
| SIGTERM | 143 (128 + 15) | [`pkg/osutil/interrupt_unix.go#L53`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/pkg/osutil/interrupt_unix.go#L53) |

## Exit Code 1 - General Errors

All server errors exit with code 1:

| Scenario | Code Reference |
| -------- | -------------- |
| Failed to create logger | [`server/etcdmain/etcd.go#L60`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/etcd.go#L60) |
| Failed to verify flags | [`server/etcdmain/etcd.go#L69`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/etcd.go#L69) |
| Discovery token already used | [`server/etcdmain/etcd.go#L141`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/etcd.go#L141) |
| Initial cluster configuration error | [`server/etcdmain/etcd.go#L155`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/etcd.go#L155) |
| Discovery failed | [`server/etcdmain/etcd.go#L157`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/etcd.go#L157) |
| Listener failed | [`server/etcdmain/etcd.go#L172`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/etcd.go#L172) |
| Failed to list data directory | [`server/etcdmain/etcd.go#L201`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/etcd.go#L201) |
| Invalid datadir (member + proxy exist) | [`server/etcdmain/etcd.go#L221`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/etcd.go#L221) |
| Unsupported architecture | [`server/etcdmain/etcd.go#L252`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/etcd.go#L252) |
| Generic fatal error | [`server/etcdmain/util.go#L34`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/util.go#L34) |
| Invalid listen-peer-urls | [`server/embed/config.go#L821`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/embed/config.go#L821) |
| Invalid listen-client-urls | [`server/embed/config.go#L830`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/embed/config.go#L830) |
| Invalid listen-client-http-urls | [`server/embed/config.go#L839`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/embed/config.go#L839) |
| Invalid initial-advertise-peer-urls | [`server/embed/config.go#L848`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/embed/config.go#L848) |
| Invalid advertise-client-urls | [`server/embed/config.go#L857`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/embed/config.go#L857) |
| Invalid listen-metrics-urls | [`server/embed/config.go#L866`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/embed/config.go#L866) |

## Exit Code 2 - Argument Errors

| Scenario | Code Reference |
| -------- | -------------- |
| Flag parsing error | [`server/etcdmain/config.go#L133`](https://github.com/etcd-io/etcd/blob/e0a72cf470756149f4f602bf89284038e6397549/server/etcdmain/config.go#L133) |
