# Production Users

This document tracks people and use cases for etcd in production. By creating a list of production use cases we hope to build a community of advisors that we can reach out to with experience using various etcd applications, operation environments, and cluster sizes. The etcd development team may reach out periodically to check-in on your experience and update this list.

## discovery.etcd.io

- *Application*: https://github.com/coreos/discovery.etcd.io
- *Launched*: Feb. 2014
- *Cluster Size*: 5 members, 5 discovery proxies
- *Order of Data Size*: 100s of Megabytes
- *Operator*: CoreOS, brandon.philips@coreos.com
- *Environment*: AWS
- *Backups*: Periodic async to S3

discovery.etcd.io is the longest continuously running etcd backed service that we know about. It is the basis of automatic cluster bootstrap and was launched in Feb. 2014: https://coreos.com/blog/etcd-0.3.0-released/.

## OpenTable

- *Application*: OpenTable internal service discovery and cluster configuration management
- *Launched*: May 2014
- *Cluster Size*: 3 members each in 6 independent clusters; approximately 50 nodes reading / writing
- *Order of Data Size*: 10s of MB
- *Operator*: OpenTable, Inc; sschlansker@opentable.com
- *Environment*: AWS, VMWare
- *Backups*: None, all data can be re-created if necessary.
