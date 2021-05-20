## Security Announcements

Join the [etcd-dev](https://groups.google.com/forum/?hl=en#!forum/etcd-dev) group for emails about security and major announcements.

## Report a Vulnerability

We’re extremely grateful for security researchers and users that report vulnerabilities to the etcd Open Source Community. All reports are thoroughly investigated by a dedicated committee of community volunteers called [Product Security Committee](security-release-process.md#product-security-committee).

To make a report, please email the private [security@etcd.io](mailto:security@etcd.io) list with the security details and the details expected for [all etcd bug reports](https://etcd.io/docs/latest/reporting_bugs/).

### When Should I Report a Vulnerability?

- When discovered a potential security vulnerability in etcd
- When unsure how a vulnerability affects etcd
- When discovered a vulnerability in another project that etcd depends on

### When Should I NOT Report a Vulnerability?

- Need help tuning etcd for security
- Need help applying security related updates
- When an issue is not security related

## Security Vulnerability Response

Each report is acknowledged and analyzed by Product Security Committee members within 3 working days. This will set off the [Security Release Process](security-release-process.md).

Any vulnerability information shared with Product Security Committee stays within etcd project and will not be disseminated to other projects unless it is necessary to get the issue fixed.

As the security issue moves from triage, to identified fix, to release planning we will keep the reporter updated.

## Public Disclosure Timing

A public disclosure date is negotiated by the etcd Product Security Committee and the bug reporter. We prefer to fully disclose the bug as soon as possible once user mitigation is available. It is reasonable to delay disclosure when the bug or the fix is not yet fully understood, the solution is not well-tested, or for vendor coordination. The timeframe for disclosure is from immediate (especially if it's already publicly known) to a few weeks. As a basic default, we expect report date to disclosure date to be on the order of 7 days. The etcd Product Security Committee holds the final say when setting a disclosure date.

## Security Audit

A third party security audit was performed by Trail of Bits, find the full report [here](SECURITY_AUDIT.pdf).

## Private Distributor List

This list provides actionable information regarding etcd security to multiple distributors. Members of the list may not use the information for anything other than fixing the issue for respective distribution's users. If you continue to leak information and break the policy outlined here, you will be removed from the list.

### Request to Join

New membership requests are sent to security@etcd.io.

File an issue [here](https://github.com/etcd-io/etcd/issues/new?template=distributors-application.md), filling in the criteria template.
