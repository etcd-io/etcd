package etcdmain

var (
	usageline = `usage: etcd [flags]
       start an etcd server

       etcd --version
       show the version of etcd

       etcd -h | --help
       show the help information about etcd
	`
	flagsline = `
member flags:

	--name 'default'
		human-readable name for this member.
	--data-dir '${name}.etcd'
		path to the data directory.
	--snapshot-count '10000'
		number of committed transactions to trigger a snapshot to disk.
	--listen-peer-urls 'http://localhost:2380,http://localhost:7001'
		list of URLs to listen on for peer traffic.
	--listen-client-urls 'http://localhost:2379,http://localhost:4001'
		list of URLs to listen on for client traffic.
	-cors ''
		comma-separated whitelist of origins for CORS (cross-origin resource sharing).


clustering flags:

	--initial-advertise-peer-urls 'http://localhost:2380,http://localhost:7001'
		list of this member's peer URLs to advertise to the rest of the cluster. 
	--initial-cluster 'default=http://localhost:2380,default=http://localhost:7001'
		initial cluster configuration for bootstrapping.
	--initial-cluster-state 'new'
		initial cluster state ('new' or 'existing').
	--initial-cluster-token 'etcd-cluster'
		initial cluster token for the etcd cluster during bootstrap.
	--advertise-client-urls 'http://localhost:2379,http://localhost:4001'
		list of this member's client URLs to advertise to the rest of the cluster.
	--discovery ''
		discovery URL used to bootstrap the cluster.
	--discovery-fallback 'proxy'
		expected behavior ('exit' or 'proxy') when discovery services fails.
	--discovery-proxy ''
		HTTP proxy to use for traffic to discovery service.


proxy flags:

	--proxy 'off'
		proxy mode setting ('off', 'readonly' or 'on').


security flags:

	--ca-file ''
		path to the client server TLS CA file.
	--cert-file ''
		path to the client server TLS cert file.
	--key-file ''
		path to the client server TLS key file.
	--peer-ca-file ''
		path to the peer server TLS CA file.
	--peer-cert-file ''
		path to the peer server TLS cert file.
	--peer-key-file ''
		path to the peer server TLS key file.


unsafe flags:

Please be CAUTIOUS to use unsafe flags because it will break the guarantee given 
by consensus protocol. 
	
	--force-new-cluster 'false'
		force to create a new one-member cluster.
`
)
