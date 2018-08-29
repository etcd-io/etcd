# etcd

[![Go Report Card](https://goreportcard.com/badge/github.com/etcd-io/etcd?style=flat-square)](https://goreportcard.com/report/github.com/etcd-io/etcd)
[![Coverage](https://codecov.io/gh/etcd-io/etcd/branch/master/graph/badge.svg)](https://codecov.io/gh/etcd-io/etcd)
[![Build Status Travis](https://img.shields.io/travis/etcd-io/etcdlabs.svg?style=flat-square&&branch=master)](https://travis-ci.com/etcd-io/etcd)
[![Build Status Semaphore](https://semaphoreci.com/api/v1/etcd-io/etcd/branches/master/shields_badge.svg)](https://semaphoreci.com/etcd-io/etcd)
[![Docs](https://readthedocs.org/projects/etcd/badge/?version=latest&style=flat-square)](https://etcd.readthedocs.io/en/latest/?badge=latest)
[![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/etcd-io/etcd)
[![Releases](https://img.shields.io/github/release/etcd-io/etcd/all.svg?style=flat-square)](https://github.com/etcd-io/etcd/releases)
[![LICENSE](https://img.shields.io/github/license/etcd-io/etcd.svg?style=flat-square)](https://github.com/etcd-io/etcd/blob/master/LICENSE)

**Note**: The `master` branch may be in an *unstable or even broken state* during development. Please use [releases][github-release] instead of the `master` branch in order to get stable binaries.

![etcd Logo](logos/etcd-horizontal-color.svg)

etcd is a distributed reliable key-value store for the most critical data of a distributed system, with a focus on being:

* *Simple*: well-defined, user-facing API (gRPC)
* *Secure*: automatic TLS with optional client cert authentication
* *Fast*: benchmarked 10,000 writes/sec
* *Reliable*: properly distributed using Raft

etcd is written in Go and uses the [Raft][raft] consensus algorithm to manage a highly-available replicated log.

etcd is used [in production by many companies](./Documentation/production-users.md), and the development team stands behind it in critical deployment scenarios, where etcd is frequently teamed with applications such as [Kubernetes][k8s], [locksmith][locksmith], [vulcand][vulcand], [Doorman][doorman], and many others. Reliability is further ensured by [**rigorous testing**](https://github.com/etcd-io/etcd/tree/master/functional).

See [etcdctl][etcdctl] for a simple command line client.

[raft]: https://raft.github.io/
[k8s]: http://kubernetes.io/
[doorman]: https://github.com/youtube/doorman
[locksmith]: https://github.com/coreos/locksmith
[vulcand]: https://github.com/vulcand/vulcand
[etcdctl]: https://github.com/etcd-io/etcd/tree/master/etcdctl

## Community meetings

*Community meeting is put on hold for now.*

etcd contributors and maintainers have bi-weekly meetings at 11:00 AM (USA Pacific) on Tuesdays.

An initial agenda will be posted to the [shared Google docs][shared-meeting-notes] a day before each meeting, and everyone is welcome to suggest additional topics or other agendas.

[shared-meeting-notes]: https://docs.google.com/document/d/16XEGyPBisZvmmoIHSZzv__LoyOeluC5a4x353CX0SIM/edit

## Getting started

### Getting etcd

The easiest way to get etcd is to use one of the pre-built release binaries which are available for OSX, Linux, Windows, and Docker on the [release page][github-release].

For more installation guides, please check out [play.etcd.io](http://play.etcd.io) and [operating etcd](https://github.com/etcd-io/etcd/tree/master/Documentation#operating-etcd-clusters).

For those wanting to try the very latest version, [build the latest version of etcd][dl-build] from the `master` branch. This first needs [*Go*](https://golang.org/) installed (version 1.11+ is required). All development occurs on `master`, including new features and bug fixes. Bug fixes are first targeted at `master` and subsequently ported to release branches, as described in the [branch management][branch-management] guide.

[github-release]: https://github.com/etcd-io/etcd/releases
[branch-management]: ./Documentation/branch_management.md
[dl-build]: ./Documentation/dl_build.md#build-the-latest-version

### Running etcd

First start a single-member cluster of etcd.

If etcd is installed using the [pre-built release binaries][github-release], run it from the installation location as below:

```bash
/tmp/etcd-download-test/etcd
```

The etcd command can be simply run as such if it is moved to the system path as below:

```bash
mv /tmp/etcd-download-test/etcd /usr/local/bin/
etcd
```

If etcd is [built from the master branch][dl-build], run it as below:

```bash
./bin/etcd
```

This will bring up etcd listening on port 2379 for client communication and on port 2380 for server-to-server communication.

Next, let's set a single key, and then retrieve it:

```
ETCDCTL_API=3 etcdctl put mykey "this is awesome"
ETCDCTL_API=3 etcdctl get mykey
```

etcd is now running and serving client requests. For more, please check out:

- [Interactive etcd playground](http://play.etcd.io)
- [Animated quick demo](./Documentation/demo.md)

### etcd TCP ports

The [official etcd ports][iana-ports] are 2379 for client requests, and 2380 for peer communication.

[iana-ports]: http://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.txt

### Running a local etcd cluster

First install [goreman](https://github.com/mattn/goreman), which manages Procfile-based applications.

Our [Procfile script](./Procfile) will set up a local example cluster. Start it with:

```bash
goreman start
```

This will bring up 3 etcd members `infra1`, `infra2` and `infra3` and etcd `grpc-proxy`, which runs locally and composes a cluster.

Every cluster member and proxy accepts key value reads and key value writes.

### Next steps

Now it's time to dig into the full etcd API and other guides.

- Read the full [documentation][fulldoc].
- Explore the full gRPC [API][api].
- Set up a [multi-machine cluster][clustering].
- Learn the [config format, env variables and flags][configuration].
- Find [language bindings and tools][integrations].
- Use TLS to [secure an etcd cluster][security].
- [Tune etcd][tuning].

[fulldoc]: ./Documentation/docs.md
[api]: ./Documentation/dev-guide/api_reference_v3.md
[clustering]: ./Documentation/op-guide/clustering.md
[configuration]: ./Documentation/op-guide/configuration.md
[integrations]: ./Documentation/integrations.md
[security]: ./Documentation/op-guide/security.md
[tuning]: ./Documentation/tuning.md

## Contact

- Mailing list: [etcd-dev](https://groups.google.com/forum/?hl=en#!forum/etcd-dev)
- IRC: #[etcd](irc://irc.freenode.org:6667/#etcd) on freenode.org
- Planning/Roadmap: [milestones](https://github.com/etcd-io/etcd/milestones), [roadmap](./ROADMAP.md)
- Bugs: [issues](https://github.com/etcd-io/etcd/issues)

## Contributing

See [CONTRIBUTING](CONTRIBUTING.md) for details on submitting patches and the contribution workflow.

## Reporting bugs

See [reporting bugs](Documentation/reporting_bugs.md) for details about reporting any issues.

## Reporting a security vulnerability

Due to their public nature, GitHub and mailing lists are NOT appropriate places for reporting vulnerabilities. Please refer to CoreOS's [security disclosure][disclosure] process when reporting issues that may be security related.

[disclosure]: https://coreos.com/security/disclosure/

### License

etcd is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
