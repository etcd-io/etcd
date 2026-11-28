---
name: etcd-cluster-maintenance
description: >-
  Check and maintain a running etcd cluster with etcdctl: endpoint health and
  status, member list/add/remove/update, leader transfer, alarms such as
  NOSPACE, compaction, defragmentation, hash checks and performance checks.
  Use when diagnosing an unhealthy etcd cluster, reclaiming space, or
  changing membership.
---

# etcd cluster maintenance

Reference: `etcdctl/README.md` (Cluster maintenance and Utility commands),
`etcdutl/README.md`.

## Health first

```bash
etcdctl endpoint health --cluster -w table
etcdctl endpoint status --cluster -w table   # leader, raft term/index, DB size, errors
etcdctl member list -w table
etcdctl alarm list
etcdctl endpoint hashkv --cluster            # compare hashes to spot divergence
```

Take a snapshot (see `etcd-backup-restore`) before any of the changes
below.

## Out of space (NOSPACE alarm)

1. Find the current revision: `etcdctl endpoint status -w json`
   (`header.revision`).
2. Compact old history: `etcdctl compaction <revision>` (add `--physical`
   to wait until old revisions are removed).
3. Defragment each member to return free pages to the filesystem:

   ```bash
   etcdctl defrag --cluster
   ```

   Defrag blocks reads and writes on a live member while it runs and is
   not replicated, so `--cluster` or a full `--endpoints` list is needed.
   Prefer one member at a time on busy clusters. For a stopped member use
   `etcdutl defrag --data-dir <dir>`.
4. Clear the alarm: `etcdctl alarm disarm`.

Compaction is irreversible: history before that revision is gone for
`get --rev` and `watch --rev`.

## Membership changes

```bash
etcdctl member add node4 --peer-urls=https://10.0.0.4:2380   # then start node4 with --initial-cluster-state existing
etcdctl member update <memberID> --peer-urls=https://10.0.0.5:2380
etcdctl member remove <memberID>
etcdctl move-leader <hex-member-id>                            # before taking the leader down
```

Change one member at a time and wait for health between steps. Removing
members below quorum makes the cluster unavailable; confirm with the user
before removing a member.

## Performance and scale

```bash
etcdctl check perf --load=s        # s, m, l, xl
etcdctl check datascale --load=s
```

Both write test data to the cluster; avoid running them against
production without approval.

## Version downgrades

`etcdctl downgrade validate <version>`, `enable <version>` and `cancel`
manage a cluster-wide downgrade; follow the release's downgrade guide.
