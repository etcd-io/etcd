# Production recommendation

The minimum recommended etcd versions to run in **production** are 3.3.18+, 3.4.2+. Refer to the [versioning policy](https://etcd.io/docs/v3.5/op-guide/versioning/) for more details.

Etcd v3.5.[0-2] versions are no longer recommended for production due to data corruption issue.

### v3.5 data corruption issue 

Running etcd v3.5.2, v3.5.1 and v3.5.0 under high load can cause a data corruption issue.
If etcd process is killed, occasionally some committed transactions are not reflected on all the members.

Recommendations if you are running v3.4.X:
* **Don't upgrade your etcd clusters to v3.5** until the problem is fixed in the upcoming v3.5.3 release.
* There are no breaking changes in API, meaning **it’s safe to let v3.5 clients (e.g. the latest Kubernetes releases) talk to v3.4 servers**.

Recommendations if you are running v3.5.0, v3.5.1, or v3.5.2:
* **Enable data corruption check** with `--experimental-initial-corrupt-check` flag. The flag is the only reliable automated way of detecting an inconsistency. This mode has seen significant usage in production and is going to be promoted as default in etcd v3.6.
* **Ensure etcd cluster is not memory pressured or sigkill interrupted**, which could lead to processes being disrupted in the middle of business logic and trigger the issue.
* **Etcd downgrade should be avoided** as they are not officially supported and clusters can be safely recovered as long as data corruption check is enabled.

If you have encountered data corruption, please follow instructions on https://etcd.io/docs/v3.5/op-guide/data_corruption/.
