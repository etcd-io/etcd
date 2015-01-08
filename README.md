# etcd

[![Build Status](https://travis-ci.org/coreos/etcd.png?branch=master)](https://travis-ci.org/coreos/etcd)
[![Docker Repository on Quay.io](https://quay.io/repository/coreos/etcd-git/status "Docker Repository on Quay.io")](https://quay.io/repository/coreos/etcd-git)

### Release Candidate Disclaimer ###

The current `master` branch of etcd is tracking the forthcoming 2.0.0 release.
We encourage users to work with the [latest release candidate](https://github.com/coreos/etcd/releases/tag/v2.0.0-rc.1), but those looking for a stable release should obtain the latest 0.4.x release, [0.4.6](https://github.com/coreos/etcd/releases/tag/v0.4.6).

Unless otherwise noted, the etcd documentation below and in [`Documentation/`](Documentation) refers to configuring and running 0.4.x releases.
Documentation related to the 2.0.0 release candidates can be found in the [`Documentation/2.0`](Documentation/2.0) directory.

![etcd Logo](logos/etcd-horizontal-color.png)

## README version 0.4.6

A highly-available key value store for shared configuration and service discovery.
etcd is inspired by [Apache ZooKeeper][zookeeper] and [doozer][doozer], with a focus on being:

* *Simple*: curl'able user facing API (HTTP+JSON)
* *Secure*: optional SSL client cert authentication
* *Fast*: benchmarked 1000s of writes/s per instance
* *Reliable*: properly distributed using Raft

etcd is written in Go and uses the [Raft][raft] consensus algorithm to manage a highly-available replicated log.

See [etcdctl][etcdctl] for a simple command line client.
Or feel free to just use curl, as in the examples below.

[zookeeper]: http://zookeeper.apache.org/
[doozer]: https://github.com/ha/doozerd
[raft]: http://raftconsensus.github.io/
[etcdctl]: https://github.com/coreos/etcd/tree/master/etcdctl

If you're considering etcd for production use, please see: [production-ready.md](./Documentation/production-ready.md)

## Getting Started

### Getting etcd

The latest release and setup instructions are available at [GitHub][github-release].

[github-release]: https://github.com/coreos/etcd/releases/

### Running etcd

First start a single-machine cluster of etcd:

```sh
./bin/etcd
```

This will bring up etcd listening on port 4001 for client communication and on port 7001 for server-to-server communication.

Next, let's set a single key, and then retrieve it:

```
curl -L http://127.0.0.1:4001/v2/keys/mykey -XPUT -d value="this is awesome"
curl -L http://127.0.0.1:4001/v2/keys/mykey
```

You have successfully started an etcd on a single machine and written a key to the store. Now it's time to dig into the full etcd API and other guides.

### Next Steps

- Explore the full [API][api].
- Set up a [multi-machine cluster][clustering].
- Learn the [config format, env variables and flags][configuration].
- Find [language bindings and tools][libraries-and-tools].
- Learn about the dashboard, lock and leader election [modules][modules].
- Use TLS to [secure an etcd cluster][security].
- [Tune etcd][tuning].
- [Upgrade from old version][upgrade].

[api]: https://github.com/coreos/etcd/blob/master/Documentation/api.md
[clustering]: https://github.com/coreos/etcd/blob/master/Documentation/clustering.md
[configuration]: https://github.com/coreos/etcd/blob/master/Documentation/configuration.md
[libraries-and-tools]: https://github.com/coreos/etcd/blob/master/Documentation/libraries-and-tools.md
[modules]: https://github.com/coreos/etcd/blob/master/Documentation/modules.md
[security]: https://github.com/coreos/etcd/blob/master/Documentation/security.md
[tuning]: https://github.com/coreos/etcd/blob/master/Documentation/tuning.md
[upgrade]: https://github.com/coreos/etcd/blob/master/Documentation/upgrade.md

## Contact

- Mailing list: [etcd-dev](https://groups.google.com/forum/?hl=en#!forum/etcd-dev)
- IRC: #[coreos](irc://irc.freenode.org:6667/#coreos) on freenode.org
- Planning/Roadmap: [milestones](https://github.com/coreos/etcd/milestones)
- Bugs: [issues](https://github.com/coreos/etcd/issues)

## Contributing

See [CONTRIBUTING](CONTRIBUTING.md) for details on submitting patches and the contribution workflow.

## Project Details

### Versioning

#### Service Versioning

etcd uses [semantic versioning](http://semver.org)
New minor versions may add additional features to the API.

You can get the version of etcd by issuing a request to /version:

```sh
curl -L http://127.0.0.1:4001/version
```

#### API Versioning

The `v2` API responses should not change after the 0.2.0 release but new features will be added over time.

The `v1` API has been deprecated and will not be supported. It will be removed in the 2.0.0 release.

#### 32-bit systems

etcd has known issues on 32-bit systems due to a bug in the Go runtime. See #[358][358] for more information.

[358]: https://github.com/coreos/etcd/issues/358

### License

etcd is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
