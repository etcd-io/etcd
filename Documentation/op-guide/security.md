---
title: Transport security model
---

etcd supports automatic TLS as well as authentication through client certificates for both clients to server as well as peer (server to server / cluster) communication. **Note that etcd doesn't enable [RBAC based authentication][auth] or the authentication feature in the transport layer by default to reduce friction for users getting started with the database. Further, changing this default would be a breaking change for the project which was established since 2013. An etcd cluster which doesn't enable security features can expose its data to any clients.**

To get up and running, first have a CA certificate and a signed key pair for one member. It is recommended to create and sign a new key pair for every member in a cluster.

For convenience, the [cfssl] tool provides an easy interface to certificate generation, and we provide an example using the tool [here][tls-setup]. Alternatively, try this [guide to generating self-signed key pairs][tls-guide].

## Basic setup

etcd takes several certificate related configuration options, either through command-line flags or environment variables:

**Client-to-server communication:**

`--cert-file=<path>`: Certificate used for SSL/TLS connections **to** etcd. When this option is set, advertise-client-urls can use the HTTPS schema.

`--key-file=<path>`: Key for the certificate. Must be unencrypted.

`--client-cert-auth`: When this is set etcd will check all incoming HTTPS requests for a client certificate signed by the trusted CA, requests that don't supply a valid client certificate will fail. If [authentication][auth] is enabled, the certificate provides credentials for the user name given by the Common Name field.

`--trusted-ca-file=<path>`: Trusted certificate authority.

`--auto-tls`: Use automatically generated self-signed certificates for TLS connections with clients.

**Peer (server-to-server / cluster) communication:**

The peer options work the same way as the client-to-server options:

`--peer-cert-file=<path>`: Certificate used for SSL/TLS connections between peers. This will be used both for listening on the peer address as well as sending requests to other peers.

`--peer-key-file=<path>`: Key for the certificate. Must be unencrypted.

`--peer-client-cert-auth`: When set, etcd will check all incoming peer requests from the cluster for valid client certificates signed by the supplied CA.

`--peer-trusted-ca-file=<path>`: Trusted certificate authority.

`--peer-auto-tls`: Use automatically generated self-signed certificates for TLS connections between peers.

If either a client-to-server or peer certificate is supplied the key must also be set. All of these configuration options are also available through the environment variables, `ETCD_CA_FILE`, `ETCD_PEER_CA_FILE` and so on.

`--cipher-suites`: Comma-separated list of supported TLS cipher suites between server/client and peers (empty will be auto-populated by Go). Available from v3.2.22+, v3.3.7+, and v3.4+.

## Example 1: Client-to-server transport security with HTTPS

For this, have a CA certificate (`ca.crt`) and signed key pair (`server.crt`, `server.key`) ready.

Let us configure etcd to provide simple HTTPS transport security step by step:

```sh
$ etcd --name infra0 --data-dir infra0 \
  --cert-file=/path/to/server.crt --key-file=/path/to/server.key \
  --advertise-client-urls=https://127.0.0.1:2379 --listen-client-urls=https://127.0.0.1:2379
```

This should start up fine and it will be possible to test the configuration by speaking HTTPS to etcd:

```sh
$ curl --cacert /path/to/ca.crt https://127.0.0.1:2379/v2/keys/foo -XPUT -d value=bar -v
```

The command should show that the handshake succeed. Since we use self-signed certificates with our own certificate authority, the CA must be passed to curl using the `--cacert` option. Another possibility would be to add the CA certificate to the system's trusted certificates directory (usually in `/etc/pki/tls/certs` or `/etc/ssl/certs`).

**OSX 10.9+ Users**: curl 7.30.0 on OSX 10.9+ doesn't understand certificates passed in on the command line.
Instead, import the dummy ca.crt directly into the keychain or add the `-k` flag to curl to ignore errors.
To test without the `-k` flag, run `open ./tests/fixtures/ca/ca.crt` and follow the prompts.
Please remove this certificate after testing!
If there is a workaround, let us know.

## Example 2: Client-to-server authentication with HTTPS client certificates

For now we've given the etcd client the ability to verify the server identity and provide transport security. We can however also use client certificates to prevent unauthorized access to etcd.

The clients will provide their certificates to the server and the server will check whether the cert is signed by the supplied CA and decide whether to serve the request.

The same files mentioned in the first example are needed for this, as well as a key pair for the client (`client.crt`, `client.key`) signed by the same certificate authority.

```sh
$ etcd --name infra0 --data-dir infra0 \
  --client-cert-auth --trusted-ca-file=/path/to/ca.crt --cert-file=/path/to/server.crt --key-file=/path/to/server.key \
  --advertise-client-urls https://127.0.0.1:2379 --listen-client-urls https://127.0.0.1:2379
```

Now try the same request as above to this server:

```sh
$ curl --cacert /path/to/ca.crt https://127.0.0.1:2379/v2/keys/foo -XPUT -d value=bar -v
```

The request should be rejected by the server:

```
...
routines:SSL3_READ_BYTES:sslv3 alert bad certificate
...
```

To make it succeed, we need to give the CA signed client certificate to the server:

```sh
$ curl --cacert /path/to/ca.crt --cert /path/to/client.crt --key /path/to/client.key \
  -L https://127.0.0.1:2379/v2/keys/foo -XPUT -d value=bar -v
```

The output should include:

```
...
SSLv3, TLS handshake, CERT verify (15):
...
TLS handshake, Finished (20)
```

And also the response from the server:

```json
{
    "action": "set",
    "node": {
        "createdIndex": 12,
        "key": "/foo",
        "modifiedIndex": 12,
        "value": "bar"
    }
}
```

Specify cipher suites to block [weak TLS cipher suites](https://github.com/etcd-io/etcd/issues/8320).

TLS handshake would fail when client hello is requested with invalid cipher suites.

For instance:

```bash
$ etcd \
  --cert-file ./server.crt \
  --key-file ./server.key \
  --trusted-ca-file ./ca.crt \
  --cipher-suites TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
```

Then, client requests must specify one of the cipher suites specified in the server:

```bash
# valid cipher suite
$ curl \
  --cacert ./ca.crt \
  --cert ./server.crt \
  --key ./server.key \
  -L [CLIENT-URL]/metrics \
  --ciphers ECDHE-RSA-AES128-GCM-SHA256

# request succeeds
etcd_server_version{server_version="3.2.22"} 1
...
```

```bash
# invalid cipher suite
$ curl \
  --cacert ./ca.crt \
  --cert ./server.crt \
  --key ./server.key \
  -L [CLIENT-URL]/metrics \
  --ciphers ECDHE-RSA-DES-CBC3-SHA

# request fails with
(35) error:14094410:SSL routines:ssl3_read_bytes:sslv3 alert handshake failure
```

## Example 3: Transport security & client certificates in a cluster

etcd supports the same model as above for **peer communication**, that means the communication between etcd members in a cluster.

Assuming we have our `ca.crt` and two members with their own keypairs (`member1.crt` & `member1.key`, `member2.crt` & `member2.key`) signed by this CA, we launch etcd as follows:


```sh
DISCOVERY_URL=... # from https://discovery.etcd.io/new

# member1
$ etcd --name infra1 --data-dir infra1 \
  --peer-client-cert-auth --peer-trusted-ca-file=/path/to/ca.crt --peer-cert-file=/path/to/member1.crt --peer-key-file=/path/to/member1.key \
  --initial-advertise-peer-urls=https://10.0.1.10:2380 --listen-peer-urls=https://10.0.1.10:2380 \
  --discovery ${DISCOVERY_URL}

# member2
$ etcd --name infra2 --data-dir infra2 \
  --peer-client-cert-auth --peer-trusted-ca-file=/path/to/ca.crt --peer-cert-file=/path/to/member2.crt --peer-key-file=/path/to/member2.key \
  --initial-advertise-peer-urls=https://10.0.1.11:2380 --listen-peer-urls=https://10.0.1.11:2380 \
  --discovery ${DISCOVERY_URL}
```

The etcd members will form a cluster and all communication between members in the cluster will be encrypted and authenticated using the client certificates. The output of etcd will show that the addresses it connects to use HTTPS.

## Example 4: Automatic self-signed transport security

For cases where communication encryption, but not authentication, is needed, etcd supports encrypting its messages with automatically generated self-signed certificates. This simplifies deployment because there is no need for managing certificates and keys outside of etcd.

Configure etcd to use self-signed certificates for client and peer connections with the flags `--auto-tls` and `--peer-auto-tls`:

```sh
DISCOVERY_URL=... # from https://discovery.etcd.io/new

# member1
$ etcd --name infra1 --data-dir infra1 \
  --auto-tls --peer-auto-tls \
  --initial-advertise-peer-urls=https://10.0.1.10:2380 --listen-peer-urls=https://10.0.1.10:2380 \
  --discovery ${DISCOVERY_URL}

# member2
$ etcd --name infra2 --data-dir infra2 \
  --auto-tls --peer-auto-tls \
  --initial-advertise-peer-urls=https://10.0.1.11:2380 --listen-peer-urls=https://10.0.1.11:2380 \
  --discovery ${DISCOVERY_URL}
```

Self-signed certificates do not authenticate identity so curl will return an error:

```sh
curl: (60) SSL certificate problem: Invalid certificate chain
```

To disable certificate chain checking, invoke curl with the `-k` flag:

```sh
$ curl -k https://127.0.0.1:2379/v2/keys/foo -Xput -d value=bar -v
```

## Notes for DNS SRV

Since v3.1.0 (except v3.2.9), discovery SRV bootstrapping authenticates `ServerName` with a root domain name from `--discovery-srv` flag. This is to avoid man-in-the-middle cert attacks, by requiring a certificate to have matching root domain name in its Subject Alternative Name (SAN) field. For instance, `etcd --discovery-srv=etcd.local` will only authenticate peers/clients when the provided certs have root domain `etcd.local` as an entry in Subject Alternative Name (SAN) field

## Notes for etcd proxy

etcd proxy terminates the TLS from its client if the connection is secure, and uses proxy's own key/cert specified in `--peer-key-file` and `--peer-cert-file` to communicate with etcd members.

The proxy communicates with etcd members through both the `--advertise-client-urls` and `--advertise-peer-urls` of a given member. It forwards client requests to etcd members’ advertised client urls, and it syncs the initial cluster configuration through etcd members’ advertised peer urls.

When client authentication is enabled for an etcd member, the administrator must ensure that the peer certificate specified in the proxy's `--peer-cert-file` option is valid for that authentication. The proxy's peer certificate must also be valid for peer authentication if peer authentication is enabled.

## Notes for TLS authentication

Since [v3.2.0](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.2.md#v320-2017-06-09), [TLS certificates get reloaded on every client connection](https://github.com/etcd-io/etcd/pull/7829). This is useful when replacing expiry certs without stopping etcd servers; it can be done by overwriting old certs with new ones. Refreshing certs for every connection should not have too much overhead, but can be improved in the future, with caching layer. Example tests can be found [here](https://github.com/coreos/etcd/blob/b041ce5d514a4b4aaeefbffb008f0c7570a18986/integration/v3_grpc_test.go#L1601-L1757).

Since [v3.2.0](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.2.md#v320-2017-06-09), [server denies incoming peer certs with wrong IP `SAN`](https://github.com/etcd-io/etcd/pull/7687). For instance, if peer cert contains any IP addresses in Subject Alternative Name (SAN) field, server authenticates a peer only when the remote IP address matches one of those IP addresses. This is to prevent unauthorized endpoints from joining the cluster. For example, peer B's CSR (with `cfssl`) is:

```json
{
  "CN": "etcd peer",
  "hosts": [
    "*.example.default.svc",
    "*.example.default.svc.cluster.local",
    "10.138.0.27"
  ],
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "C": "US",
      "L": "CA",
      "ST": "San Francisco"
    }
  ]
}
```

when peer B's actual IP address is `10.138.0.2`, not `10.138.0.27`. When peer B tries to join the cluster, peer A will reject B with the error `x509: certificate is valid for 10.138.0.27, not 10.138.0.2`, because B's remote IP address does not match the one in Subject Alternative Name (SAN) field.

Since [v3.2.0](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.2.md#v320-2017-06-09), [server resolves TLS `DNSNames` when checking `SAN`](https://github.com/etcd-io/etcd/pull/7767). For instance, if peer cert contains only DNS names (no IP addresses) in Subject Alternative Name (SAN) field, server authenticates a peer only when forward-lookups (`dig b.com`) on those DNS names have matching IP with the remote IP address. For example, peer B's CSR (with `cfssl`) is:

```json
{
  "CN": "etcd peer",
  "hosts": [
    "b.com"
  ],
```

when peer B's remote IP address is `10.138.0.2`. When peer B tries to join the cluster, peer A looks up the incoming host `b.com` to get the list of IP addresses (e.g. `dig b.com`). And rejects B if the list does not contain the IP `10.138.0.2`, with the error `tls: 10.138.0.2 does not match any of DNSNames ["b.com"]`.

Since [v3.2.2](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.2.md#v322-2017-07-07), [server accepts connections if IP matches, without checking DNS entries](https://github.com/etcd-io/etcd/pull/8223). For instance, if peer cert contains IP addresses and DNS names in Subject Alternative Name (SAN) field, and the remote IP address matches one of those IP addresses, server just accepts connection without further checking the DNS names. For example, peer B's CSR (with `cfssl`) is:

```json
{
  "CN": "etcd peer",
  "hosts": [
    "invalid.domain",
    "10.138.0.2"
  ],
```

when peer B's remote IP address is `10.138.0.2` and `invalid.domain` is a invalid host. When peer B tries to join the cluster, peer A successfully authenticates B, since Subject Alternative Name (SAN) field has a valid matching IP address. See [issue#8206](https://github.com/etcd-io/etcd/issues/8206) for more detail.

Since [v3.2.5](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.2.md#v325-2017-08-04), [server supports reverse-lookup on wildcard DNS `SAN`](https://github.com/etcd-io/etcd/pull/8281). For instance, if peer cert contains only DNS names (no IP addresses) in Subject Alternative Name (SAN) field, server first reverse-lookups the remote IP address to get a list of names mapping to that address (e.g. `nslookup IPADDR`). Then accepts the connection if those names have a matching name with peer cert's DNS names (either by exact or wildcard match). If none is matched, server forward-lookups each DNS entry in peer cert (e.g. look up `example.default.svc` when the entry is `*.example.default.svc`), and accepts connection only when the host's resolved addresses have the matching IP address with the peer's remote IP address. For example, peer B's CSR (with `cfssl`) is:

```json
{
  "CN": "etcd peer",
  "hosts": [
    "*.example.default.svc",
    "*.example.default.svc.cluster.local"
  ],
```

when peer B's remote IP address is `10.138.0.2`. When peer B tries to join the cluster, peer A reverse-lookup the IP `10.138.0.2` to get the list of host names. And either exact or wildcard match the host names with peer B's cert DNS names in Subject Alternative Name (SAN) field. If none of reverse/forward lookups worked, it returns an error `"tls: "10.138.0.2" does not match any of DNSNames ["*.example.default.svc","*.example.default.svc.cluster.local"]`. See [issue#8268](https://github.com/etcd-io/etcd/issues/8268) for more detail.

[v3.3.0](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.3.md) adds [`etcd --peer-cert-allowed-cn`](https://github.com/etcd-io/etcd/pull/8616) flag to support [CN(Common Name)-based auth for inter-peer connections](https://github.com/etcd-io/etcd/issues/8262). Kubernetes TLS bootstrapping involves generating dynamic certificates for etcd members and other system components (e.g. API server, kubelet, etc.). Maintaining different CAs for each component provides tighter access control to etcd cluster but often tedious. When `--peer-cert-allowed-cn` flag is specified, node can only join with matching common name even with shared CAs. For example, each member in 3-node cluster is set up with CSRs (with `cfssl`) as below:

```json
{
  "CN": "etcd.local",
  "hosts": [
    "m1.etcd.local",
    "127.0.0.1",
    "localhost"
  ],
```

```json
{
  "CN": "etcd.local",
  "hosts": [
    "m2.etcd.local",
    "127.0.0.1",
    "localhost"
  ],
```

```json
{
  "CN": "etcd.local",
  "hosts": [
    "m3.etcd.local",
    "127.0.0.1",
    "localhost"
  ],
```

Then only peers with matching common names will be authenticated if `--peer-cert-allowed-cn etcd.local` is given. And nodes with different CNs in CSRs or different `--peer-cert-allowed-cn` will be rejected:

```bash
$ etcd --peer-cert-allowed-cn m1.etcd.local

I | embed: rejected connection from "127.0.0.1:48044" (error "CommonName authentication failed", ServerName "m1.etcd.local")
I | embed: rejected connection from "127.0.0.1:55702" (error "remote error: tls: bad certificate", ServerName "m3.etcd.local")
```

Each process should be started with:

```bash
etcd --peer-cert-allowed-cn etcd.local

I | pkg/netutil: resolving m3.etcd.local:32380 to 127.0.0.1:32380
I | pkg/netutil: resolving m2.etcd.local:22380 to 127.0.0.1:22380
I | pkg/netutil: resolving m1.etcd.local:2380 to 127.0.0.1:2380
I | etcdserver: published {Name:m3 ClientURLs:[https://m3.etcd.local:32379]} to cluster 9db03f09b20de32b
I | embed: ready to serve client requests
I | etcdserver: published {Name:m1 ClientURLs:[https://m1.etcd.local:2379]} to cluster 9db03f09b20de32b
I | embed: ready to serve client requests
I | etcdserver: published {Name:m2 ClientURLs:[https://m2.etcd.local:22379]} to cluster 9db03f09b20de32b
I | embed: ready to serve client requests
I | embed: serving client requests on 127.0.0.1:32379
I | embed: serving client requests on 127.0.0.1:22379
I | embed: serving client requests on 127.0.0.1:2379
```

[v3.2.19](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.2.md) and [v3.3.4](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.3.md) fixes TLS reload when [certificate SAN field only includes IP addresses but no domain names](https://github.com/etcd-io/etcd/issues/9541). For example, a member is set up with CSRs (with `cfssl`) as below:

```json
{
  "CN": "etcd.local",
  "hosts": [
    "127.0.0.1"
  ],
```

In Go, server calls `(*tls.Config).GetCertificate` for TLS reload if and only if server's `(*tls.Config).Certificates` field is not empty, or `(*tls.ClientHelloInfo).ServerName` is not empty with a valid SNI from the client. Previously, etcd always populates `(*tls.Config).Certificates` on the initial client TLS handshake, as non-empty. Thus, client was always expected to supply a matching SNI in order to pass the TLS verification and to trigger `(*tls.Config).GetCertificate` to reload TLS assets.

However, a certificate whose SAN field does [not include any domain names but only IP addresses](https://github.com/etcd-io/etcd/issues/9541) would request `*tls.ClientHelloInfo` with an empty `ServerName` field, thus failing to trigger the TLS reload on initial TLS handshake; this becomes a problem when expired certificates need to be replaced online.

Now, `(*tls.Config).Certificates` is created empty on initial TLS client handshake, first to trigger `(*tls.Config).GetCertificate`, and then to populate rest of the certificates on every new TLS connection, even when client SNI is empty (e.g. cert only includes IPs).

## Notes for Host Whitelist

`etcd --host-whitelist` flag specifies acceptable hostnames from HTTP client requests. Client origin policy protects against ["DNS Rebinding"](https://en.wikipedia.org/wiki/DNS_rebinding) attacks to insecure etcd servers. That is, any website can simply create an authorized DNS name, and direct DNS to `"localhost"` (or any other address). Then, all HTTP endpoints of etcd server listening on `"localhost"` becomes accessible, thus vulnerable to DNS rebinding attacks. See [CVE-2018-5702](https://bugs.chromium.org/p/project-zero/issues/detail?id=1447#c2) for more detail.

Client origin policy works as follows:

1. If client connection is secure via HTTPS, allow any hostnames.
2. If client connection is not secure and `"HostWhitelist"` is not empty, only allow HTTP requests whose Host field is listed in whitelist.

Note that the client origin policy is enforced whether authentication is enabled or not, for tighter controls.

By default, `etcd --host-whitelist` and `embed.Config.HostWhitelist` are set *empty* to allow all hostnames. Note that when specifying hostnames, loopback addresses are not added automatically. To allow loopback interfaces, add them to whitelist manually (e.g. `"localhost"`, `"127.0.0.1"`, etc.).

## Frequently asked questions

### I'm seeing a SSLv3 alert handshake failure when using TLS client authentication?

The `crypto/tls` package of `golang` checks the key usage of the certificate public key before using it.
To use the certificate public key to do client auth, we need to add `clientAuth` to `Extended Key Usage` when creating the certificate public key.

Here is how to do it:

Add the following section to openssl.cnf:

```
[ ssl_client ]
...
  extendedKeyUsage = clientAuth
...
```

When creating the cert be sure to reference it in the `-extensions` flag:

```
$ openssl ca -config openssl.cnf -policy policy_anything -extensions ssl_client -out certs/machine.crt -infiles machine.csr
```

### With peer certificate authentication I receive "certificate is valid for 127.0.0.1, not $MY_IP"
Make sure to sign the certificates with a Subject Name the member's public IP address. The `etcd-ca` tool for example provides an `--ip=` option for its `new-cert` command.

The certificate needs to be signed for the member's FQDN in its Subject Name, use Subject Alternative Names (short IP SANs) to add the IP address. The `etcd-ca` tool provides `--domain=` option for its `new-cert` command, and openssl can make [it][alt-name] too.

### Does etcd encrypt data stored on disk drives?
No. etcd doesn't encrypt key/value data stored on disk drives. If a user need to encrypt data stored on etcd, there are some options:
* Let client applications encrypt and decrypt the data
* Use a feature of underlying storage systems for encrypting stored data like [dm-crypt]

### I’m seeing a log warning that "directory X exist without recommended permission -rwx------"
When etcd create certain new directories it sets file permission to 700 to prevent unprivileged access as possible. However, if user has already created a directory with own preference, etcd uses the existing directory and logs a warning message if the permission is different than 700.

[cfssl]: https://github.com/cloudflare/cfssl
[tls-setup]: ../../hack/tls-setup
[tls-guide]: https://github.com/coreos/docs/blob/master/os/generate-self-signed-certificates.md
[alt-name]: http://wiki.cacert.org/FAQ/subjectAltName
[auth]: authentication.md
[dm-crypt]: https://en.wikipedia.org/wiki/Dm-crypt
