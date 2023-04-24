# Change logs

## Production recommendation

The minimum recommended etcd versions to run in **production** are v3.4.22+ and v3.5.6+. Refer to the [versioning policy](https://etcd.io/docs/v3.5/op-guide/versioning/) for more details.

### v3.5 data corruption issue 

Running etcd v3.5.2, v3.5.1 and v3.5.0 under high load can cause a data corruption issue.
If etcd process is killed, occasionally some committed transactions are not reflected on all the members.
Recommendation is to upgrade to v3.5.4+.

If you have encountered data corruption, please follow instructions on https://etcd.io/docs/v3.5/op-guide/data_corruption/.

## Change log rules
1. Each patch release only includes changes against previous patch release.
For example, the change log of v3.5.5 should only include items which are new to v3.5.4.
2. For the first release (e.g. 3.4.0, 3.5.0, 3.6.0, 4.0.0 etc.) for each minor or major 
version, it only includes changes which are new to the first release of previous minor
or major version. For example, v3.5.0 should only include items which are new to v3.4.0,
and v3.6.0 should only include items which are new to v3.5.0.
