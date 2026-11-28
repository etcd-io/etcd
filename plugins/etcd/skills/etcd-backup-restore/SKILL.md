---
name: etcd-backup-restore
description: >-
  Back up and restore etcd: snapshot save with etcdctl, verifying snapshots
  with etcdutl snapshot status and hashkv, and restoring members into a new
  cluster with etcdutl snapshot restore (initial-cluster flags,
  bump-revision and mark-compacted). Use when taking etcd backups or
  recovering a cluster, including Kubernetes control planes.
---

# etcd backup and restore

Reference: `etcdctl/README.md` (SNAPSHOT), `etcdutl/README.md`.

## Back up

```bash
etcdctl snapshot save snapshot-$(date +%Y%m%d-%H%M).db
etcdutl snapshot status snapshot-*.db -w table     # hash, revision, total keys, size
```

Point `etcdctl` at one healthy member (with the usual TLS flags). Store
snapshots off the node and test restores regularly.

## Restore into a new cluster

Run on each member with that member's name and peer URL, using the same
token and `--initial-cluster`:

```bash
etcdutl snapshot restore snapshot.db \
  --name sshot1 \
  --initial-cluster 'sshot1=http://127.0.0.1:12380,sshot2=http://127.0.0.1:22380,sshot3=http://127.0.0.1:32380' \
  --initial-cluster-token etcd-cluster-1 \
  --initial-advertise-peer-urls http://127.0.0.1:12380 \
  --data-dir /var/lib/etcd-restored
```

Then start each `etcd` with the restored data dir and matching
`--name`/peer/client URLs. A restore creates a new cluster identity; do
not mix restored and old members.

## Options that matter

- `--data-dir` / `--wal-dir`: target directories (default `<name>.etcd`).
  Restore into an empty directory, never over a live member's data.
- `--skip-hash-check`: needed only when the file was copied from a data
  directory (`member/snap/db`) rather than saved with `snapshot save`.
- `--bump-revision N --mark-compacted`: raise the revision after restore so
  watchers (e.g. Kubernetes informers) don't see revisions go backwards;
  `--mark-compacted` is required whenever `--bump-revision > 0`.

## Kubernetes control planes

Stop the API servers (or move static pod manifests aside) before swapping
in a restored data dir, restore with `--bump-revision` and
`--mark-compacted`, then bring etcd up before the API servers. Confirm
each step with the user; this replaces cluster state.

## Offline checks

```bash
etcdutl hashkv /var/lib/etcd/member/snap/db
etcdutl defrag --data-dir /var/lib/etcd            # only while etcd is stopped
```
