# Production recommendation

The minimum recommended etcd versions to run in **production** are 3.3.18+, 3.4.2+, v3.5.3+. Refer to the [versioning policy](https://etcd.io/docs/v3.5/op-guide/versioning/) for more details.

### v3.5 data corruption issue 

Running etcd v3.5.2, v3.5.1 and v3.5.0 under high load can cause a data corruption issue.
If etcd process is killed, occasionally some committed transactions are not reflected on all the members.
Recommendation is to upgrade to v3.5.3.

If you have encountered data corruption, please follow instructions on https://etcd.io/docs/v3.5/op-guide/data_corruption/.
