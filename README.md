# etcd

[![Build Status](https://travis-ci.org/coreos/etcd.png?branch=master)](https://travis-ci.org/coreos/etcd)
[![Docker Repository on Quay.io](https://quay.io/repository/coreos/etcd-git/status "Docker Repository on Quay.io")](https://quay.io/repository/coreos/etcd-git)

![etcd Logo](logos/etcd-horizontal-color.png)

etcd is a distributed, consistent key-value store for shared configuration and service discovery, with a focus on being:

* *Simple*: curl'able user facing API (HTTP+JSON)
* *Secure*: optional SSL client cert authentication
* *Fast*: benchmarked 1000s of writes/s per instance
* *Reliable*: properly distributed using Raft

etcd is written in Go and uses the [Raft][raft] consensus algorithm to manage a highly-available replicated log.

See [etcdctl][etcdctl] for a simple command line client.
Or feel free to just use curl, as in the examples below.

[raft]: http://raftconsensus.github.io/
[etcdctl]: https://github.com/coreos/etcd/tree/master/etcdctl

If you're considering etcd for production use, please see: [production-ready.md](./Documentation/production-ready.md)

## Getting Started

### Getting etcd

The easiest way to get etcd is to install one of the pre-built binaries from the tagged releases: instructions are available on [GitHub][github-release].

For those wanting to try the very latest version, you can build the latest version of etcd from the `master` branch.
All development occurs on `master`, including new features and bug fixes.
Bug fixes are first targeted at `master` and subsequently ported to release branches, as described in the [branch management][branch-management] guide.

[github-release]: https://github.com/coreos/etcd/releases/
[branch-management]: ./Documentation/branch_management.md

### Running etcd

First start a single-member cluster of etcd:

```sh
./bin/etcd
```

This will bring up etcd listening on port 2379 for client communication and on port 2380 for server-to-server communication.

Next, let's set a single key, and then retrieve it:

```
curl -L http://127.0.0.1:2379/v2/keys/mykey -XPUT -d value="this is awesome"
curl -L http://127.0.0.1:2379/v2/keys/mykey
```

You have successfully started an etcd and written a key to the store.

### Running local etcd cluster

First install [goreman](https://github.com/mattn/goreman), which manages Procfile-based applications.

Our [Procfile script](./Procfile) will set up a local example cluster. You can start it with:

```sh
goreman start
```

This will bring up 3 etcd members `infra1`, `infra2` and `infra3` and etcd proxy `proxy`, which runs locally and composes a cluster.

You can write a key to the cluster and retrieve the value back from any member or proxy.

### Next Steps

Now it's time to dig into the full etcd API and other guides.

- Explore the full [API][api].
- Set up a [multi-machine cluster][clustering].
- Learn the [config format, env variables and flags][configuration].
- Find [language bindings and tools][libraries-and-tools].
- Use TLS to [secure an etcd cluster][security].
- [Tune etcd][tuning].
- [Upgrade from 0.4.6 to 2.0.0][upgrade].

[api]: ./Documentation/api.md
[clustering]: ./Documentation/clustering.md
[configuration]: ./Documentation/configuration.md
[libraries-and-tools]: ./Documentation/libraries-and-tools.md
[security]: ./Documentation/security.md
[tuning]: ./Documentation/tuning.md
[upgrade]: ./tools/etcd-migrate/README.md

## Contact

- Mailing list: [etcd-dev](https://groups.google.com/forum/?hl=en#!forum/etcd-dev)
- IRC: #[etcd](irc://irc.freenode.org:6667/#etcd) on freenode.org
- Planning/Roadmap: [milestones](https://github.com/coreos/etcd/milestones), [roadmap](./ROADMAP.md)
- Bugs: [issues](https://github.com/coreos/etcd/issues)

## Contributing

See [CONTRIBUTING](CONTRIBUTING.md) for details on submitting patches and the contribution workflow.

## Reporting bugs

See [reporting bugs](Documentation/reporting_bugs.md) for details about reporting any issue you may encounter..

## Project Details

### Versioning

#### Service Versioning

etcd uses [semantic versioning](http://semver.org)
New minor versions may add additional features to the API.

You can get the version of etcd by issuing a request to /version:

```sh
curl -L http://127.0.0.1:2379/version
```

#### API Versioning

The `v2` API responses should not change after the 2.0.0 release but new features will be added over time.

#### 32-bit systems

etcd has known issues on 32-bit systems due to a bug in the Go runtime. See #[358][358] for more information.

[358]: https://github.com/coreos/etcd/issues/358

### License

etcd is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
