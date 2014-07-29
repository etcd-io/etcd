# etcd

README version 0.4.6

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
[etcdctl]: http://github.com/coreos/etcdctl/

If you're considering etcd for production use, please see: [production-ready.md](./Documentation/production-ready.md)

## Getting Started

### Getting etcd

The latest release and setup instructions are available at [GitHub][github-release].

[github-release]: https://github.com/coreos/etcd/releases/


### Building

You can build etcd from source:

```sh
git clone https://github.com/coreos/etcd
cd etcd
./build
```

This will generate a binary called `./bin/etcd`.

_NOTE_: you need go 1.2+. Please check your installation with

```
go version
```

See the [development tools documentation][development-tools] for alternative build methods like using Vagrant.

[development-tools]: https://github.com/coreos/etcd/blob/master/Documentation/development-tools.md

### Running

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
- Planning/Roadmap: [milestones](https://github.com/coreos/etcd/issues/milestones)
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

Clients are encouraged to use the `v2` API. The `v1` API will not change.

The `v2` API responses should not change after the 0.2.0 release but new features will be added over time.

During the pre-v1.0.0 series of releases we may break the API as we fix bugs and get feedback.

#### 32bit systems

etcd has known issues on 32bit systems. We plan to address these issues after the 0.4 release.

### License

etcd is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
