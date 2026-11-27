## Security Announcements

Join the [etcd-dev](https://groups.google.com/g/etcd-dev) group for emails about security and major announcements.

## Report a Vulnerability

We’re extremely grateful for security researchers and users that report vulnerabilities to the etcd Open Source Community. All reports are thoroughly investigated by a dedicated committee of community volunteers called [Product Security Committee](security-release-process.md#product-security-committee).

To make a report, please fill out the [Security Vulnerability Reporting Form](https://github.com/etcd-io/etcd/security/advisories/new) on GitHub.

### When Should I Report a Vulnerability?

- You have found a vulnerability in an current supported version and patch release of etcd
- You have found a vulnerability in a library that etcd depends on, that may affect etcd
- The vulnerability is a security issue according to our [Threat Model](../THREAT_MODEL.md)

### When Should I NOT Report a Vulnerability?

- If you need help tuning etcd for security, or applying security related updates
- If you find a vulnerability in an EOL or unpatched version of etcd and have not checked it against an updated one
- If the vulnerability is clearly outside our [Threat Model](../THREAT_MODEL.md)

## Security Vulnerability Response

Each report is acknowledged and analyzed by Product Security Committee members within 5 working days. This will set off the [Security Release Process](security-release-process.md).

Any vulnerability information shared with Product Security Committee stays within etcd project and will not be disseminated to other projects unless it is necessary to get the issue fixed.

As the security issue moves from triage, to identified fix, to release planning we will keep the reporter updated.

## Public Disclosure Timing

A public disclosure date is negotiated by the etcd Product Security Committee and the bug reporter. We prefer to fully disclose the bug as soon as possible once user mitigation is available. It is reasonable to delay disclosure when the bug or the fix is not yet fully understood, the solution is not well-tested, or for vendor coordination. The timeframe for disclosure is from immediate (especially if it's already publicly known) to a few weeks. As a basic default, we expect report date to disclosure date to be on the order of 1-3 weeks depending on the nature of the vulnerability and concurrent project timing. The etcd Product Security Committee holds the final say when setting a disclosure date.

## Security Audit

A third party security audit was performed by Trail of Bits, find the full report [here](SECURITY_AUDIT.pdf).
A third party fuzzing audit was performed by Ada Logics, find the full report [here](FUZZING_AUDIT_2022.PDF).

