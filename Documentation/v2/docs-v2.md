# Documentation

etcd is a distributed key-value store designed to reliably and quickly preserve and provide access to critical data. It enables reliable distributed coordination through distributed locking, leader elections, and write barriers. An etcd cluster is intended for high availability and permanent data storage and retrieval.

This is the etcd v2 documentation set. For more recent versions, please see the [etcd v3 guides][etcd-v3].

## Communicating with etcd v2

Reading and writing into the etcd keyspace is done via a simple, RESTful HTTP API, or using language-specific libraries that wrap the HTTP API with higher level primitives.

### Reading and Writing

 - [Client API Documentation][api]
 - [Libraries, Tools, and Language Bindings][libraries]
 - [Admin API Documentation][admin-api]
 - [Members API][members-api]

### Security, Auth, Access control

 - [Security Model][security]
 - [Auth and Security][auth_api]
 - [Authentication Guide][authentication]

## etcd v2 Cluster Administration

Configuration values are distributed within the cluster for your applications to read. Values can be changed programmatically and smart applications can reconfigure automatically. You'll never again have to run a configuration management tool on every machine in order to change a single config value.

### General Info

 - [etcd Proxies][proxy]
 - [Production Users][production-users]
 - [Admin Guide][admin_guide]
 - [Configuration Flags][configuration]
 - [Frequently Asked Questions][faq]

### Initial Setup

 - [Tuning etcd Clusters][tuning]
 - [Discovery Service Protocol][discovery_protocol]
 - [Running etcd under Docker][docker_guide]

### Live Reconfiguration

 - [Runtime Configuration][runtime-configuration]

### Debugging etcd

 - [Metrics Collection][metrics]
 - [Error Code][errorcode]
 - [Reporting Bugs][reporting_bugs]

### Migration

 - [Upgrade etcd to 2.3][upgrade_2_3]
 - [Upgrade etcd to 2.2][upgrade_2_2]
 - [Upgrade to etcd 2.1][upgrade_2_1]
 - [Snapshot Migration (0.4.x to 2.x)][04_to_2_snapshot_migration]
 - [Backward Compatibility][backward_compatibility]


[etcd-v3]: ../docs.md
[api]: api.md
[libraries]: libraries-and-tools.md
[admin-api]: other_apis.md
[members-api]: members_api.md
[security]: security.md
[auth_api]: auth_api.md
[authentication]: authentication.md
[proxy]: proxy.md
[production-users]: production-users.md
[admin_guide]: admin_guide.md
[configuration]: configuration.md
[faq]: faq.md
[tuning]: tuning.md
[discovery_protocol]: discovery_protocol.md
[docker_guide]: docker_guide.md
[runtime-configuration]: runtime-configuration.md
[metrics]: metrics.md
[errorcode]: errorcode.md
[reporting_bugs]: reporting_bugs.md
[upgrade_2_3]: upgrade_2_3.md
[upgrade_2_2]: upgrade_2_2.md
[upgrade_2_1]: upgrade_2_1.md
[04_to_2_snapshot_migration]: 04_to_2_snapshot_migration.md
[backward_compatibility]: backward_compatibility.md
