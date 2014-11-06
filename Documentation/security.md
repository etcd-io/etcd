# Etcd security model

Etcd supports SSL/TLS as well as authentication through client certificates, both for clients to server as well as peer (server to server / cluster) communication.

To get up and running you first need to have a CA certificate and a signed key pair for your node. It is recommended to create and sign a new key pair for every node in a cluster.

For convenience the [etcd-ca](https://github.com/coreos/etcd-ca) tool provides an easy interface to certificate generation, alternatively this site provides a good reference on how to generate self-signed key pairs:

http://www.g-loaded.eu/2005/11/10/be-your-own-ca/

## Basic setup

Etcd takes several certificate related configuration options, either through command-line flags or environment variables:

**Client-to-server communication:**

`--cert-file=<path>`: Certificate used for SSL/TLS connections **to** etcd. When this option is set, you can reach etcd through HTTPS - for example at `https://127.0.0.1:4001`  
`--key-file=<path>`: Key for the certificate. Must be unencrypted.  
`--ca-file=<path>`: When this is set etcd will check all incoming HTTPS requests for a client certificate signed by the supplied CA, requests that don't supply a valid client certificate will fail.

**Peer (server-to-server / cluster) communication:**

The peer options work the same way as the client-to-server options:

`--peer-cert-file=<path>`: Certificate used for SSL/TLS connections between peers. This will be used both for listening on the peer address as well as sending requests to other peers.  
`--peer-key-file=<path>`: Key for the certificate. Must be unencrypted.  
`--peer-ca-file=<path>`: When set, etcd will check all incoming peer requests from the cluster for valid client certificates signed by the supplied CA.

If either a client-to-server or peer certificate is supplied the key must also be set. All of these configuration options are also available through the environment variables, `ETCD_CA_FILE`, `ETCD_PEER_CA_FILE` and so on.

## Example 1: Client-to-server transport security with HTTPS

For this you need your CA certificate (`ca.crt`) and signed key pair (`server.crt`, `server.key`) ready. If you just want to test the functionality, there are example certificates provided in the [etcd git repository](https://github.com/coreos/etcd/tree/master/fixtures/ca) (namely `server.crt` and `server.key.insecure`).

Assuming you have these files ready, let's configure etcd to use them to provide simple HTTPS transport security.

```sh
etcd -name machine0 -data-dir machine0 -cert-file=/path/to/server.crt -key-file=/path/to/server.key
```

This should start up fine and you can now test the configuration by speaking HTTPS to etcd:

```sh
curl --cacert /path/to/ca.crt https://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar -v
```

You should be able to see the handshake succeed. Because we use self-signed certificates with our own certificate authorities you need to provide the CA to curl using the `--cacert` option. Another possibility would be to add your CA certificate to the trusted certificates on your system (usually in `/etc/ssl/certs`).

**OSX 10.9+ Users**: curl 7.30.0 on OSX 10.9+ doesn't understand certificates passed in on the command line.
Instead you must import the dummy ca.crt directly into the keychain or add the `-k` flag to curl to ignore errors.
If you want to test without the `-k` flag run `open ./fixtures/ca/ca.crt` and follow the prompts.
Please remove this certificate after you are done testing!
If you know of a workaround let us know.

## Example 2: Client-to-server authentication with HTTPS client certificates

For now we've given the etcd client the ability to verify the server identity and provide transport security. We can however also use client certificates to prevent unauthorized access to etcd.

The clients will provide their certificates to the server and the server will check whether the cert is signed by the supplied CA and decide whether to serve the request.

You need the same files mentioned in the first example for this, as well as a key pair for the client (`client.crt`, `client.key`) signed by the same certificate authority.

```sh
etcd -name machine0 -data-dir machine0 -ca-file=/path/to/ca.crt -cert-file=/path/to/server.crt -key-file=/path/to/server.key
```

Notice that the addition of the `-ca-file` option automatically enables client certificate checking.


Now try the same request as above to this server:

```sh
curl --cacert /path/to/ca.crt https://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar -v
```

The request should be rejected by the server:

```
...
routines:SSL3_READ_BYTES:sslv3 alert bad certificate
...
```

To make it succeed, we need to give the CA signed client certificate to the server:

```sh
curl --cacert /path/to/ca.crt --cert /path/to/client.crt --key /path/to/client.key -L https://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar -v
```

You should able to see:

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

## Example 3: Transport security & client certificates in a cluster

Etcd supports the same model as above for **peer communication**, that means the communication between etcd nodes in a cluster.

Assuming we have our `ca.crt` and two nodes with their own keypairs (`node1.crt` & `node1.key`, `node2.crt` & `node2.key`) signed by this CA, we launch etcd as follows:


```sh
DISCOVERY_URL=... # from https://discovery.etcd.io/new

# Node1
etcd -name node1 -data-dir node1 -ca-file=/path/to/ca.crt -cert-file=/path/to/node1.crt -key-file=/path/to/node1.key -peer-addr ${node1_public_ip}:7001 -discovery ${DISCOVERY_URL}

# Node2
etcd -name node2 -data-dir node2 -ca-file=/path/to/ca.crt -cert-file=/path/to/node2.crt -key-file=/path/to/node2.key -peer-addr ${node2_public_ip}:7001 -discovery ${DISCOVERY_URL}
```

The etcd nodes will form a cluster and all communication between nodes in the cluster will be encrypted and authenticated using the client certificates. You will see in the output of etcd that the addresses it connects to use HTTPS.

## Frequently Asked Questions

### I'm seeing a SSLv3 alert handshake failure when using SSL client authentication?

The `crypto/tls` package of `golang` checks the key usage of the certificate public key before using it.
To use the certificate public key to do client auth, we need to add `clientAuth` to `Extended Key Usage` when creating the certificate public key.

Here is how to do it:

Add the following section to your openssl.cnf:

```
[ ssl_client ]
...
  extendedKeyUsage = clientAuth
...
```

When creating the cert be sure to reference it in the `-extensions` flag:

```
openssl ca -config openssl.cnf -policy policy_anything -extensions ssl_client -out certs/machine.crt -infiles machine.csr
```

### With peer certificate authentication I receive "certificate is valid for 127.0.0.1, not $MY_IP"
Make sure that you sign your certificates with a Subject Name your node's public IP address. The `etcd-ca` tool for example provides an `--ip=` option for its `new-cert` command.

If you need your certificate to be signed for your node's FQDN in its Subject Name then you could use Subject Alternative Names (short IP SNAs) to add your IP address. This is not [currently supported](https://github.com/coreos/etcd-ca/issues/29) by `etcd-ca` but can be done [with openssl](http://wiki.cacert.org/FAQ/subjectAltName).
