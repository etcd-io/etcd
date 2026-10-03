
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

Client and peer private keys and certificates are assumed to be properly protected.
Attack scenarios that presume a stolen valid certificate, or attacker-controlled infrastructure inside the perimeter (such as reverse DNS), are outside this threat model.

### The Client-to-Server Boundary

etcd clients communicate with etcd Servers over Port 2379.
This boundary requires **mTLS encryption**.
Any client request must prove its identity at the transport layer using client certificates.

Traffic that has passed client mTLS authentication is **trusted input**.
Crashes, memory exhaustion, or resource leaks triggered by malformed or high-volume requests from an authenticated client are robustness defects, not vulnerabilities.
A vulnerability report against this boundary must demonstrate reachability by an unauthenticated actor, or in effect exceeding the sender's existing privileges.

### The Peer-to-Peer Boundary

etcd Server members communicate with other cluster members over Port 2380 to run Raft consensus.
This boundary must be strictly limited to authorized cluster members using dedicated, private peer certificates (mTLS).

Data arriving over the authenticated peer transport — Raft payloads, snapshot streams, lease-forwarding bodies, and peer HTTP headers — is **trusted input**.
Peer HTTP handlers rely on transport-level authentication rather than per-request authentication by design.
Panics or unbounded allocations reachable only from an authenticated peer are robustness defects.
A vulnerability report against this boundary must demonstrate exploitability by a node that is not an authorized cluster member.

### The Authentication & Authorization Boundary

etcd's built-in role-based access control is an **optional, secondary** control layered behind mTLS.
It is disabled by default, and major consumers (such as the Kubernetes API server) rely on transport security and network isolation instead.
Timing side channels and name-enumeration leaks behind the transport boundary are hardening concerns, not vulnerabilities.
Watch-stream permission revocation is **eventually consistent by design**; established streams may deliver events for a bounded period after revocation.
A report is actionable only if revocation never takes effect, or if a new stream can be opened after revocation.

### The Host Execution Boundary

etcd Server is compiled as a pure, statically linked Go binary (`CGO_ENABLED=0`).
It does not dynamically load or link to dynamic system libraries (such as host C libraries or host transport encryption libraries) at runtime.

### The Data Storage Boundary

etcd Server writes data to the local storage subsystem exactly as received from the client.
Data protection at rest is a client-side responsibility (e.g., client envelope encryption) or must be managed via disk filesystem encryption.

### The Build & Release Boundary

Scripts and tooling under `scripts/`, `tools/`, `hack/`, and `.github/` execute only in trusted, access-controlled build, test, and release environments.
They are not part of the shipped product's attack surface.
Unpinned tool downloads, missing checksums, predictable temporary paths, and workflow-input interpolation are supply-chain hardening items.
Fixes are welcome as normal contributions, not security reports.

---

## Component Scope

The boundaries above apply to the **etcd server binary**, the official client library, and the production command-line tools (`etcdctl`, `etcdutl`).

The following components are best-effort and are not covered by the security-response process:

- `grpc-proxy`
- the `cache` package
- everything under `contrib/`

Defects in these components are handled as normal issues and pull requests, without advisories or CVE assignment.

---

## Non-Default & Test-Only Configuration

Diagnostic facilities are intended for trusted debugging contexts only:

- pprof endpoints (`--enable-pprof`)
- the expvar endpoint (`/debug/vars`)
- verbose logging (`--log-level=debug`), which also exposes gRPC tracing
- distributed tracing (`--enable-distributed-tracing`)

Except for `/debug/vars`, these facilities are **disabled by default**. Enabling them is an explicit operator decision that knowingly expands the attack surface. The `/debug/vars` endpoint is protected by mTLS like other client APIs, and does not expose sensitive information.

Reports predicated on a non-default flag, or on a documented operator-overridable default, are configuration hardening.
Listener schemes that exist to support etcd's test suites (`unix://`, `unixs://`) are not production configurations and are out of scope.

---

## Reporting

A finding is a security vulnerability only if it crosses one of the boundaries above:

- reachable by an unauthenticated actor, or
- exceeding an authenticated actor's existing privileges, and
- in a production-scoped component, under production-relevant configuration.

Everything else — robustness defects behind trusted boundaries, hardening of best-effort components, build tooling, and non-default features — is welcome through normal issues and pull requests.
The project routinely accepts and backports such fixes; "not a vulnerability" never means "do not report".

Authentication bypass, privilege escalation, data corruption triggerable by an unprivileged actor, and remote code execution always warrant a report through the [security-disclosure process](security/README.md).
