.. _faq:


General
#######


What is etcd?
=============

etcd is a consistent distributed key-value store. Mainly used as a separate coordination service, in distributed systems. And designed to hold small amounts of data that can fit entirely in memory.


How to pronounce etcd?
======================

etcd is pronounced ``/ˈɛtsiːdiː/``, and means distributed ``etc`` directory.


Do clients have to send requests to the etcd leader?
====================================================

`Raft <https://raft.github.io/raft.pdf>`_ is leader-based; the leader handles all client requests which need cluster consensus. However, the client does not need to know which node is the leader. Any request that requires consensus sent to a follower is automatically forwarded to the leader. Requests that do not require consensus (e.g., serialized reads) can be processed by any cluster member.


Configuration
#############


Difference between listen-<client,peer>-urls, advertise-client-urls or initial-advertise-peer-urls?
===================================================================================================

``--listen-client-urls`` and ``--listen-peer-urls`` specify the local addresses etcd server binds to for accepting incoming connections. To listen on a port for all interfaces, specify ``0.0.0.0`` as the listen IP address.

``--advertise-client-urls`` and ``--initial-advertise-peer-urls`` specify the addresses etcd clients or other etcd members should use to contact the etcd server. The advertise addresses must be reachable from the remote machines. Do not advertise addresses like ``localhost`` or ``0.0.0.0`` for a production setup since these addresses are unreachable from remote machines.


Changing "listen-peer-urls" or "initial-advertise-peer-urls" does not update advertised peer URLs in member list output?
========================================================================================================================

A member's advertised peer URLs come from ``--initial-advertise-peer-urls`` on initial cluster boot. Changing the listen peer URLs or the initial advertise peers after booting the member won't affect the exported advertise peer URLs (e.g. ``etcdctl member list`` output remains the same), since changes must go through quorum to avoid membership configuration split brain. Use `etcdctl member update` to update a member's peer URLs.
