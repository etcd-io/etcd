# etcd Threat Model

This document defines the security assumptions and trust boundaries of the etcd project.

Automated vulnerability scanners and security researchers MUST evaluate any security concern against these baseline boundaries.

---

## Security Boundaries & Assumptions

etcd Server is a consistent, distributed key-value storage designed to operate as a secure backend.
The baseline security boundaries are structured as follows:

### The Network Boundary

etcd Server assumes it is deployed within a strictly isolated, private network segment.
It **must not** be exposed to untrusted networks or the public internet.
Both the **etcd Client** and the **etcd Server** reside inside this protected perimeter.

### The Client-to-Server Boundary

etcd clients communicate with etcd Servers over Port 2379.
This boundary requires **mTLS encryption**.
Any client request must prove its identity at the transport layer using client certificates.

### The Peer-to-Peer Boundary

etcd Server members communicate with other cluster members over Port 2380 to run Raft consensus.
This boundary must be strictly limited to authorized cluster members using dedicated, private peer certificates (mTLS).

### The Host Execution Boundary

etcd Server is compiled as a pure, statically linked Go binary (`CGO_ENABLED=0`).
It does not dynamically load or link to dynamic system libraries (such as host C libraries or host transport encryption libraries) at runtime.

### The Data Storage Boundary

etcd Server writes data to the local storage subsystem exactly as received from the client.
Data protection at rest is a client-side responsibility (e.g., client envelope encryption) or must be managed via disk filesystem encryption.
