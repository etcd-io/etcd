---
name: etcdctl-keyspace
description: >-
  Read and write etcd data with etcdctl (v3 API): put, get with prefixes and
  revisions, delete, watch, transactions, leases, distributed locks and
  elections, connection and TLS flags, and output formats. Use when
  inspecting or changing keys in an etcd cluster, including Kubernetes'
  etcd.
---

# etcdctl key-value work

Reference: `etcdctl/README.md`.

## Connecting

```bash
export ETCDCTL_ENDPOINTS=https://10.0.0.1:2379,https://10.0.0.2:2379
export ETCDCTL_CACERT=/etc/etcd/ca.pem ETCDCTL_CERT=/etc/etcd/client.pem ETCDCTL_KEY=/etc/etcd/client-key.pem
export ETCDCTL_DIAL_TIMEOUT=3s
```

Any global flag can be set as `ETCDCTL_<FLAG>` (upper case, dashes to
underscores). The v3 API is the default; only releases before v3.4 need
`ETCDCTL_API=3`.

## Read

```bash
etcdctl get foo
etcdctl get --prefix /registry/pods/ --keys-only
etcdctl get --prefix app/ --limit 10 --sort-by MODIFY --order DESCEND
etcdctl get foo --rev 42                 # historical value (until compacted)
etcdctl get foo -w json                  # simple | json | protobuf | fields | table
etcdctl get foo --consistency s          # serializable (local, possibly stale)
```

Prefer `--keys-only` and `--limit` on large keyspaces before dumping
values. Kubernetes stores protobuf values under `/registry/`; they are
not human-readable with plain `get`.

## Write and delete

```bash
etcdctl put foo bar
etcdctl put foo bar --lease 32695410dcc0ca06
etcdctl del foo
etcdctl del --prefix app/tmp/            # prints the count deleted
```

Writing to a Kubernetes etcd bypasses the API server's validation. Get the
user's explicit confirmation before any `put` or `del` there.

## Watch

```bash
etcdctl watch --prefix app/ --prev-kv
etcdctl watch foo --rev 100              # replay from a revision
```

## Transactions

```bash
etcdctl txn <<<'mod("key1") > "0"

put key1 "overwrote-key1"

put key1 "created-key1"
put key2 "some extra key"
'
```

Blocks are: compares, blank line, success ops, blank line, failure ops.
`etcdctl txn -i` prompts for each block interactively.

## Leases

```bash
etcdctl lease grant 60                   # lease 32695410dcc0ca06 granted with TTL(60s)
etcdctl put session/worker-1 up --lease 32695410dcc0ca06
etcdctl lease keep-alive 32695410dcc0ca06
etcdctl lease timetolive 32695410dcc0ca06 --keys
etcdctl lease revoke 32695410dcc0ca06    # deletes attached keys
```

## Locks and elections

```bash
etcdctl lock mylock ./critical-job.sh    # ETCD_LOCK_KEY / ETCD_LOCK_REV set for the command
etcdctl lock --ttl 30 mylock
etcdctl elect my-election candidate-a
```

All commands exit 0 on success and non-zero on failure.
