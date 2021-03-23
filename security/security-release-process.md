# Security Release Process

etcd is a growing community of volunteers, users, and vendors. The etcd community has adopted this security disclosures and response policy to ensure we responsibly handle critical issues.

## Product Security Committee (PSC)

Security vulnerabilities should be handled quickly and sometimes privately. The primary goal of this process is to reduce the total time users are vulnerable to publicly known exploits.

The PSC is responsible for organizing the entire response including internal communication and external disclosure but will need help from relevant developers and release leads to successfully run this process.

The initial PSC will consist of volunteers who have been involved in the initial discussion:

- Brandon Philips (**[@philips](https://github.com/philips)**) [4096R/154343260542DF34]
- Gyuho Lee (**[@gyuho](https://github.com/gyuho)**)
- Joe Betz (**[@jpbetz](https://github.com/jpbetz)**)
- Sahdev Zala (**[@spzala](https://github.com/spzala)**)
- Sam Batschelet (**[@hexfusion](https://github.com/hexfusion)**)
- Xiang Li (**[@xiang90](https://github.com/xiang90)**)

The PSC members will share various tasks as listed below:

- Triage: make sure the people who should be in "the know" (aka notified) are notified, also responds to issues that are not actually issues and let the etcd maintainers know that. This person is the escalation path for a bug if it is one. 
- Infra: make sure we can test the fixes appropriately.
- Disclosure: handles public messaging around the bug. Documentation on how to upgrade. Changelog. Explaining to public the severity. notifications of bugs sent to mailing lists etc. Requests CVEs.
- Release: Create new release addressing a security fix.

### Contacting the Product Security Committee

Contact the team by sending email to [security@etcd.io](mailto:security@etcd.io)

### Product Security Committee Membership

#### Joining

The PSC should be consist of 2-4 members. New potential members to the PSC can express their interest to the PSC members. These individuals can be nominated by PSC members or etcd maintainers.

If representation changes due to job shifts then PSC members are encouraged to grow the team or replace themselves through mentoring new members.

##### Product Security Committee Lazy Consensus Selection

Selection of new members will be done by lazy consensus amongst members for adding new people with fallback on majority vote.

#### Stepping Down

Members may step down at any time and propose a replacement from existing active contributors of etcd.

#### Responsibilities

- Members must remain active and responsive.
- Members taking an extended leave of two weeks or more should coordinate with other members to ensure the role is adequately staffed during the leave.
- Members going on leave for 1-3 months may identify a temporary replacement.
- Members of a role should remove any other members that have not communicated a leave of absence and either cannot be reached for more than 1 month or are not fulfilling their documented responsibilities for more than 1 month. This may be done through a super-majority vote of members.

## Disclosures

### Private Disclosure Processes

The etcd Community asks that all suspected vulnerabilities be privately and responsibly disclosed as explained in the [README](README.md).

### Public Disclosure Processes

If anyone knows of a publicly disclosed security vulnerability please IMMEDIATELY email [security@etcd.io](mailto:security@etcd.io) to inform the PSC about the vulnerability so they may start the patch, release, and communication process.

If possible the PSC will ask the person making the public report if the issue can be handled via a private disclosure process. If the reporter denies the PSC will move swiftly with the fix and release process. In extreme cases GitHub can be asked to delete the issue but this generally isn't necessary and is unlikely to make a public disclosure less damaging.

## Patch, Release, and Public Communication

For each vulnerability, the PSC members will coordinate to create the fix and release, and sending email to the rest of the community. 

All of the timelines below are suggestions and assume a Private Disclosure.
The PSC drives the schedule using their best judgment based on severity,
development time, and release work. If the PSC is dealing with
a Public Disclosure all timelines become ASAP. If the fix relies on another
upstream project's disclosure timeline, that will adjust the process as well.
We will work with the upstream project to fit their timeline and best protect
etcd users.

### Fix Team Organization

These steps should be completed within the first 24 hours of Disclosure.

- The PSC will work quickly to identify relevant engineers from the affected projects and packages and CC those engineers into the disclosure thread. These selected developers are the Fix Team. A best guess is to invite all maintainers.

### Fix Development Process

These steps should be completed within the 1-7 days of Disclosure.

- The PSC and the Fix Team will create a [CVSS](https://www.first.org/cvss/specification-document) using the [CVSS Calculator](https://www.first.org/cvss/calculator/3.0) to determine the effect and severity of the bug. The PSC makes the final call on the calculated risk; it is better to move quickly than make the perfect assessment.
- The PSC will request a [CVE](https://cveform.mitre.org/).
- The Fix Team will notify the PSC that work on the fix branch is complete once there are LGTMs on all commits from one or more maintainers.

If the CVSS score is under ~4.0
([a low severity score](https://www.first.org/cvss/specification-document#i5)) or the assessed risk is low the Fix Team can decide to slow the release process down in the face of holidays, developer bandwidth, etc.

Note: CVSS is convenient but imperfect. Ultimately, the PSC has discretion on classifying the severity of a vulnerability.

The severity of the bug and related handling decisions must be discussed on the security@etcd.io mailing list.

### Fix Disclosure Process

With the Fix Development underway, the PSC needs to come up with an overall communication plan for the wider community. This Disclosure process should begin after the Fix Team has developed a Fix or mitigation so that a realistic timeline can be communicated to users.

**Fix Release Day** (Completed within 1-21 days of Disclosure)

- The PSC will cherry-pick the patches onto the master branch and all relevant release branches. The Fix Team will `lgtm` and `approve`.
- The etcd maintainers will merge these PRs as quickly as possible.
- The PSC will ensure all the binaries are built, publicly available, and functional.
- The PSC will announce the new releases, the CVE number, severity, and impact, and the location of the binaries to get wide distribution and user action. As much as possible this announcement should be actionable, and include any mitigating steps users can take prior to upgrading to a fixed version. The recommended target time is 4pm UTC on a non-Friday weekday. This means the announcement will be seen morning Pacific, early evening Europe, and late evening Asia. The announcement will be sent via the following channels:
  - etcd-dev@googlegroups.com
  - [Kubernetes announcement slack channel](https://kubernetes.slack.com/messages/C9T0QMNG4)
  - [etcd slack channel](https://kubernetes.slack.com/messages/C3HD8ARJ5)

## Retrospective

These steps should be completed 1-3 days after the Release Date. The retrospective process [should be blameless](https://landing.google.com/sre/book/chapters/postmortem-culture.html).

- The PSC will send a retrospective of the process to etcd-dev@googlegroups.com including details on everyone involved, the timeline of the process, links to relevant PRs that introduced the issue, if relevant, and any critiques of the response and release process.
- The PSC and Fix Team are also encouraged to send their own feedback on the process to etcd-dev@googlegroups.com. Honest critique is the only way we are going to get good at this as a community.
