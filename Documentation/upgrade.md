# Upgrading an Existing Cluster

etcd clusters can be upgraded by doing a rolling upgrade or all at once. We make every effort to test this process, but please be sure to backup your data [by etcd-dump](https://github.com/AaronO/etcd-dump), or make a copy of data directory beforehand.

## Upgrade Process

- Stop the old etcd processes
- Upgrade the etcd binary
- Restart the etcd instance using the original --name, --address, --peer-address and --data-dir.

## Rolling Upgrade

During an upgrade, etcd clusters are designed to continue working in a mix of old and new versions. It's recommended to converge on the new version quickly. Using new API features before the entire cluster has been upgraded is only supported as a best effort. Each instance's version can be found with `curl http://127.0.0.1:4001/version`.

## All at Once

If downtime is not an issue, the easiest way to upgrade your cluster is to shutdown all of the etcd instances and restart them with the new binary. The current state of the cluster is saved to disk and will be loaded into the cluster when it restarts.
