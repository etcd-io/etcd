# Documentation

etcd is a distributed key-value store designed to reliably and quickly preserve and provide access to critical data. It enables reliable distributed coordination through distributed locking, leader elections, and write barriers. An etcd cluster is intended for high availability and permanent data storage and retrieval.

## Getting started

New etcd users and developers should get started by [downloading and building][download_build] etcd. Once you have etcd, follow this [quick demo][demo] to see the basics of creating and working with an etcd cluster.

## Developing with etcd

The easiest way to get started using etcd as a distributed key-value store for your applications is to [set up a local cluster][local_cluster].

 - [Setting up local clusters][local_cluster]
 - [Interacting with etcd][interacting]
 - [API references][api_ref]
 - [gRPC gateway][api_grpc_gateway]

## Operating etcd clusters

Administrators who need to create reliable and scalable key-value stores for the developers they support should begin with a [cluster on multiple machines][clustering].

 - [Setting up clusters][clustering]
 - [Run etcd clusters inside containers][container]
 - [Configuration][conf]
 - [Security][security]
 - Monitoring
 - [Maintenance][maintenance]
 - [Understand failures][failures]
 - [Disaster recovery][recovery]
 - [Performance][performance]

## Learning

To learn more about the concepts and internals behind etcd, read the following pages:

 - Why etcd
 - Concepts
 - Internals
 - [Glossary][glossary]

## Upgrading and compatibility

 - [Migrate applications from using API v2 to API v3][v2_migration]

## Troubleshooting

[api_ref]: dev-guide/api_reference_v3.md
[api_grpc_gateway]: dev-guide/api_grpc_gateway.md
[clustering]: op-guide/clustering.md
[conf]: op-guide/configuration.md
[demo]: demo.md
[download_build]: dl_build.md
[failures]: op-guide/failures.md
[glossary]: learning/glossary.md
[interacting]: dev-guide/interacting_v3.md
[local_cluster]: dev-guide/local_cluster.md
[performance]: op-guide/performance.md
[recovery]: op-guide/recovery.md
[maintenance]: op-guide/maintenance.md
[security]: op-guide/security.md
[v2_migration]: op-guide/v2-migration.md
[container]: op-guide/container.md
