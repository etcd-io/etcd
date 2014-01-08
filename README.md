# etcd

README version 0.2.0

[![Build Status](https://travis-ci.org/coreos/etcd.png)](https://travis-ci.org/coreos/etcd)

A highly-available key value store for shared configuration and service discovery.
etcd is inspired by zookeeper and doozer, with a focus on:

* Simple: curl'able user facing API (HTTP+JSON)
* Secure: optional SSL client cert authentication
* Fast: benchmarked 1000s of writes/s per instance
* Reliable: Properly distributed using Raft

Etcd is written in Go and uses the [Raft][raft] consensus algorithm to manage a highly-available replicated log.

See [etcdctl][etcdctl] for a simple command line client.
Or feel free to just use curl, as in the examples below.

[raft]: https://github.com/coreos/go-raft
[etcdctl]: http://github.com/coreos/etcdctl/

## Contact

- Mailing list: http://coreos.com/lists/etcd-dev/
- IRC: #coreos on irc.freenode.net
- Planning/Roadmap: https://trello.com/b/OiEbU547/etcd
- Bugs: https://github.com/coreos/etcd/issues


## Getting Started

### Getting etcd

The latest release and setup instructions are available at [Github][github-release].

[github-release]: https://github.com/coreos/etcd/releases/


### Building

You can build etcd from source:

```sh
git clone https://github.com/coreos/etcd
cd etcd
./build
```

This will generate a binary in the base directory called `./etcd`.

_NOTE_: you need go 1.1+. Please check your installation with

```
go version
```

### Running

First start a single machine cluster of etcd:

```sh
./etcd
```

This will bring up etcd listening on port 4001 for client communication and on port 7001 for server-to-server communication.

Next lets set a single key and then retrieve it:

```
curl -L http://127.0.0.1:4001/v2/keys/mykey -XPUT -d value="this is awesome"
curl -L http://127.0.0.1:4001/v2/keys/mykey
```

You have successfully started an etcd on a single machine and written a key to the store. Now it time to dig into the full etcd API and other guides.

### Next Steps

- Explore the full [API][api.md].
- Setup a [multi-machine cluster][clustering.md].
- Learn the [config format, env variables and flags][configuration.md].
- Find [language bindings and tools][libraries-and-tools.md].
- Learn about the dashboard, lock and leader election [modules][modules.md].
- Use TLS to [secure an etcd cluster][security.md].
- [Tune etcd][tuning.md].

[api.md]: https://github.com/coreos/etcd/blob/master/Documentation/api.md
[clustering.md]: https://github.com/coreos/etcd/blob/master/Documentation/clustering.md
[configuration.md]: https://github.com/coreos/etcd/blob/master/Documentation/configuration.md
[libraries-and-tools.md]: https://github.com/coreos/etcd/blob/master/Documentation/libraries-and-tools.md
[modules.md]: https://github.com/coreos/etcd/blob/master/Documentation/modules.md
[security.md]: https://github.com/coreos/etcd/blob/master/Documentation/security.md
[tuning.md]: https://github.com/coreos/etcd/blob/master/Documentation/tuning.md

## Contributing

See [CONTRIBUTING](https://github.com/coreos/etcd/blob/master/CONTRIBUTING.md) for details on submitting patches and contacting developers via IRC and mailing lists.

## Project Details

### Versioning

#### Service Versioning

etcd uses [semantic versioning][semver].
New minor versions may add additional features to the API.

You can get the version of etcd by issuing a request to /version:

```sh
curl -L http://127.0.0.1:4001/version
```

[semver]: http://semver.org/

#### API Versioning

Clients are encouraged to use the `v2` API. The `v1` API will not change.

The `v2` API responses should not change after the 0.2.0 release but new features will be added over time.

During the pre-v1.0.0 series of releases we may break the API as we fix bugs and get feedback.

### License

etcd is under the Apache 2.0 license. See the [LICENSE][license] file for details.

[license]: https://github.com/coreos/etcd/blob/master/LICENSE
