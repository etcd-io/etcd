## Advanced Usage

### Transport security with HTTPS

Etcd supports SSL/TLS and client cert authentication for clients to server, as well as server to server communication.

First, you need to have a CA cert `clientCA.crt` and signed key pair `client.crt`, `client.key`.
This site has a good reference for how to generate self-signed key pairs:
http://www.g-loaded.eu/2005/11/10/be-your-own-ca/

For testing you can use the certificates in the `fixtures/ca` directory.

Let's configure etcd to use this keypair:

```sh
./etcd -f -name machine0 -data-dir machine0 -cert-file=./fixtures/ca/server.crt -key-file=./fixtures/ca/server.key.insecure
```

There are a few new options we're using:

* `-f` - forces a new machine configuration, even if an existing configuration is found. (WARNING: data loss!)
* `-cert-file` and `-key-file` specify the location of the cert and key files to be used for for transport layer security between the client and server.

You can now test the configuration using HTTPS:

```sh
curl --cacert ./fixtures/ca/server-chain.pem https://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar -v
```

You should be able to see the handshake succeed.

**OSX 10.9+ Users**: curl 7.30.0 on OSX 10.9+ doesn't understand certificates passed in on the command line.
Instead you must import the dummy ca.crt directly into the keychain or add the `-k` flag to curl to ignore errors.
If you want to test without the `-k` flag run `open ./fixtures/ca/ca.crt` and follow the prompts.
Please remove this certificate after you are done testing!
If you know of a workaround let us know.

```
...
SSLv3, TLS handshake, Finished (20):
...
```

And also the response from the etcd server:

```json
{
    "action": "set",
    "key": "/foo",
    "modifiedIndex": 3,
    "value": "bar"
}
```


### Authentication with HTTPS client certificates

We can also do authentication using CA certs.
The clients will provide their cert to the server and the server will check whether the cert is signed by the CA and decide whether to serve the request.

```sh
./etcd -f -name machine0 -data-dir machine0 -ca-file=./fixtures/ca/ca.crt -cert-file=./fixtures/ca/server.crt -key-file=./fixtures/ca/server.key.insecure
```

```-ca-file``` is the path to the CA cert.

Try the same request to this server:

```sh
curl --cacert ./fixtures/ca/server-chain.pem https://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar -v
```

The request should be rejected by the server.

```
...
routines:SSL3_READ_BYTES:sslv3 alert bad certificate
...
```

We need to give the CA signed cert to the server.

```sh
curl --key ./fixtures/ca/server2.key.insecure --cert ./fixtures/ca/server2.crt --cacert ./fixtures/ca/server-chain.pem -L https://127.0.0.1:4001/v2/keys/foo -XPUT -d value=bar -v
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

### Why SSLv3 alert handshake failure when using SSL client auth?

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
