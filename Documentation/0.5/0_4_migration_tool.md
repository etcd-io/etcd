## etcd 0.4.x -> 0.5.0 Data Migration Tool

### Upgrading from 0.4.x

Between 0.4.x and 0.5, the on-disk data formats have changed. In order to allow users to convert to 0.5, a migration tool is provided.

In the early 0.5.0-alpha series, we're providing this tool early to encourage adoption. However, before 0.5.0-release, etcd will autodetect the 0.4.x data dir upon upgrade and automatically update the data too (while leaving a backup, in case of emergency).

### Data Migration Tips

* Keep the environment variables and etcd instance flags the same (much as [the upgrade document](../upgrade.md) suggests), particularly `--name`/`ETCD_NAME`.
* Don't change the cluster configuration. If there's a plan to add or remove machines, it's probably best to arrange for that after the migration, rather than before or at the same time.

### Running the tool

The tool can be run via:
```sh
./bin/etcd-migrate --data-dir=<PATH TO YOUR DATA>
```

It should autodetect everything and convert the data-dir to be 0.5 compatible. It does not remove the 0.4.x data, and is safe to convert multiple times; the 0.5 data will be overwritten. Recovering the disk space once everything is settled is covered later in the document.

If, however, it complains about autodetecting the name (which can happen, depending on how the cluster was configured), you need to supply the name of this particular node. This is equivalent to the `--name` flag (or `ETCD_NAME` variable) that etcd was run with, which can also be found by accessing the self api, eg:

```sh
curl -L http://127.0.0.1:4001/v2/stats/self
```

Where the `"name"` field is the name of the local machine.

Then, run the migration tool with

```sh
./bin/etcd-migrate --data-dir=<PATH TO YOUR DATA> --name=<NAME>
```

And the tool should migrate successfully. If it still has an error at this time, it's a failure or bug in the tool and it's worth reporting a bug.

### Recovering Disk Space

If the conversion has completed, the entire cluster is running on something 0.5-based, and the disk space is important, the following command will clear 0.4.x data from the data-dir:

```sh
rm -ri snapshot conf log
```

It will ask before every deletion, but these are the 0.4.x files and will not affect the working 0.5 data.
