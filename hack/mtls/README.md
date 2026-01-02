# mTLS Development Cluster

This sets up an etcd cluster with full mutual TLS authentication for both peer-to-peer and client-to-server communication, with support for CA certificate rotation.

## Prerequisites

Install the required tools:

```bash
go install github.com/cloudflare/cfssl/cmd/cfssl@latest
go install github.com/cloudflare/cfssl/cmd/cfssljson@latest
go install github.com/mattn/goreman@latest
```

Ensure `~/go/bin` is in your PATH.

## Quick Start

```bash
# Build etcd (from repo root)
make build

# Generate certs and start cluster
cd hack/mtls
make start
```

## Manual Steps

```bash
# Generate certificates
make

# Start cluster
goreman -f Procfile start
```

## Using etcdctl with mTLS

```bash
../../bin/etcdctl --endpoints=https://localhost:2379 \
  --cacert=certs/ca.pem \
  --cert=certs/client.pem \
  --key=certs/client-key.pem \
  member list
```

## Cluster Endpoints

| Node   | Client URL               | Peer URL                |
|--------|--------------------------|-------------------------|
| infra1 | https://localhost:2379   | https://localhost:2380  |
| infra2 | https://localhost:12379  | https://localhost:12380 |
| infra3 | https://localhost:22379  | https://localhost:22380 |

## mTLS Configuration

- **Client mTLS**: `--client-cert-auth` + `--trusted-ca-file`
- **Peer mTLS**: `--peer-client-cert-auth` + `--peer-trusted-ca-file`
- **CA Reload**: `--peer-tls-reload-ca` + `--client-tls-reload-ca` + `--tls-ca-reload-interval`

All clients must present a valid certificate signed by the CA to connect.

## CA Rotation Test

To test CA certificate rotation:

```bash
# Interactive mode (prompts between phases)
./test-rotation.sh

# Non-interactive mode (runs all phases automatically)
./test-rotation.sh -y

# Or via make
make test-rotation
```

This runs an automated 4-phase test with a 4-node cluster:

1. **Phase 1**: Starts 3-node cluster (node1, node2, node3) with CA1
2. **Phase 2**: Adds CA2 to trust bundle, adds node4 with CA2-signed cert
3. **Phase 3**: Rotates node2 and node3 certs to CA2
4. **Phase 4**: Removes CA1, verifies node1 (still on CA1) is rejected
   - Nodes 2, 3, 4 maintain quorum (3 of 4 members)

The test validates that:
- New CAs can be added to the trust bundle without restart
- Nodes with certs from the new CA can join the cluster
- Existing nodes can rotate their certs to the new CA
- Old CAs can be removed, rejecting nodes still using old certs
- Cluster maintains availability during CA rotation (with proper quorum)
