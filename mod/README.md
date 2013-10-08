## Etcd modules

etcd modules (mods) are higher order pieces of functionality that only
speak to the client etcd API and are presented in the `/mod` HTTP path
of the etcd service.

The basic idea is that etcd can ship things like dashboards, master
election APIs and other helpful services that would normally have to
stand up and talk to an etcd cluster directly in the binary. It is a
convienence and hopefully eases complexity in deployments.
