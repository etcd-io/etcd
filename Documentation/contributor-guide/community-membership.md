# Community membership

This doc outlines the various responsibilities of contributor roles in etcd. 

| Role       | Responsibilities                             | Requirements                                                  | Defined by                           |
|------------|----------------------------------------------|---------------------------------------------------------------|--------------------------------------|
| Member     | Active contributor in the community          | Sponsored by 2 reviewers and multiple contributions           | etcd GitHub org member               |
| Reviewer   | Review contributions from other members      | History of review and authorship                              | [MAINTAINERS] file reviewer entry    |
| Maintainer | Set direction and priorities for the project | Demonstrated responsibility and excellent technical judgement | [MAINTAINERS] file maintainers entry |

## New contributors

New contributors should be welcomed to the community by existing members,
helped with PR workflow, and directed to relevant documentation and
communication channels.

## Established community members

Established community members are expected to demonstrate their adherence to the
principles in this document, familiarity with project organization, roles,
policies, procedures, conventions, etc., and technical and/or writing ability.
Role-specific expectations, responsibilities, and requirements are enumerated
below.

## Member

Members are continuously active contributors in the community.  They can have
issues and PRs assigned to them. Members are expected to remain active 
contributors to the community.

**Defined by:** Member of the etcd GitHub organization.

### Requirements

- Enabled [two-factor authentication] on their GitHub account
- Have made multiple contributions to the project or community.  Contribution may include, but is not limited to:
    - Authoring or reviewing PRs on GitHub. At least one PR must be **merged**.
    - Filing or commenting on issues on GitHub
    - Contributing to community discussions (e.g. meetings, Slack, email discussion
      forums, Stack Overflow)
- Subscribed to [etcd-dev@googlegroups.com]
- Have read the [contributor guide]
- Sponsored by one active maintainer or two reviewers.
    - Sponsors must be from multiple member companies to demonstrate integration across community.
    - With no objections from other maintainers
- Open a [membership nomination] issue against the etcd-io/etcd repo
    - Ensure your sponsors are @mentioned on the issue
    - Make sure that the list of contributions included is representative of your work on the project.
- Members can be removed by a supermajority of the maintainers or can resign by notifying
  the maintainers.

### Responsibilities and privileges

- Responsive to issues and PRs assigned to them
- Granted "triage access" to etcd project
- Active owner of code they have contributed (unless ownership is explicitly transferred)
    - Code is well tested
    - Tests consistently pass
    - Addresses bugs or issues discovered after code is accepted

**Note:** members who frequently contribute code are expected to proactively
perform code reviews and work towards becoming a *reviewer*.

## Reviewers

Reviewers are contributors who have demonstrated greater skill in
reviewing the code from other contributors. They are knowledgeable about both 
the codebase and software engineering principles. Their LGTM counts towards
merging a code change into the project. A reviewer is generally on the ladder towards
maintainership. 

**Defined by:** *reviewers* entry in the [MAINTAINERS] file.

### Requirements

- member for at least 3 months.
- Primary reviewer for at least 5 PRs to the codebase.
- Reviewed or contributed at least 20 substantial PRs to the codebase.
- Knowledgeable about the codebase.
- Sponsored by two active maintainers.
    - Sponsors must be from multiple member companies to demonstrate integration across community.
    - With no objections from other maintainers
- Reviewers can be removed by a supermajority of the maintainers or can resign by notifying
  the maintainers.

### Responsibilities and privileges

- Code reviewer status may be a precondition to accepting large code contributions
- Responsible for project quality control via code reviews
    - Focus on code quality and correctness, including testing and factoring
    - May also review for more holistic issues, but not a requirement
- Expected to be responsive to review requests
- Assigned PRs to review related to area of expertise
- Assigned test bugs related to area of expertise
- Granted "triage access" to etcd project

## Maintainers

Maintainers are first and foremost contributors that have shown they
are committed to the long term success of a project. Maintainership is about building
trust with the current maintainers and being a person that they can
depend on to make decisions in the best interest of the project in a consistent manner.

**Defined by:** *maintainers* entry in the [MAINTAINERS] file.

### Requirements

- Deep understanding of the technical goals and direction of the project
- Deep understanding of the technical domain of the project
- Sustained contributions to design and direction by doing all of:
    - Authoring and reviewing proposals
    - Initiating, contributing and resolving discussions (emails, GitHub issues, meetings)
    - Identifying subtle or complex issues in designs and implementation PRs
- Directly contributed to the project through implementation and / or review
- Sponsored by two active maintainers and elected by supermajority
    - Sponsors must be from multiple member companies to demonstrate integration across community.
- To become a maintainer send an email with your candidacy to [etcd-maintainers-private@googlegroups.com]
    - Ensure your sponsors are @mentioned on the email
    - Include a list of contributions representative of your work on the project.
    - Existing maintainers vote will privately and respond to the email with either acceptance or with feedback for suggested improvement.
- With your membership approved you are expected to:
  - Open a PR and add an entry to the [MAINTAINERS] file
  - Subscribe to [etcd-maintainers@googlegroups.com] and [etcd-maintainers-private@googlegroups.com]
  - Request to join to [etcd-maintainer teams of etcd organization of GitHub](https://github.com/orgs/etcd-io/teams/maintainers-etcd)
  - Request to join to the private slack channel for etcd maintainers on [kubernetes slack](http://slack.kubernetes.io/)
  - Request access to etcd-development GCP project where we publish releases
  - Request access to passwords shared between maintainers

### Responsibilities and privileges

- Make and approve technical design decisions
- Set technical direction and priorities
- Define milestones and releases
- Mentor and guide reviewers, and contributors to the project.
- Participate when called upon in the [security disclosure and release process]
- Ensure continued health of the project
    - Adequate test coverage to confidently release
    - Tests are passing reliably (i.e. not flaky) and are fixed when they fail
- Ensure a healthy process for discussion and decision making is in place.
- Work with other maintainers to maintain the project's overall health and success holistically

### Retiring

Life priorities, interests, and passions can change. Maintainers can retire and
move to [emeritus maintainers]. If a maintainer needs to step down, they should
inform other maintainers, if possible, help find someone to pick up the related
work. At the very least, ensure the related work can be continued. Afterward
they can remove themselves from list of existing maintainers.

If a maintainer has not been performing their duties for period of 12 months,
they can be removed by other maintainers. In that case inactive maintainer will
be first notified via an email. If situation doesn't improve, they will be
removed. If an emeritus maintainer wants to regain an active role, they can do
so by renewing their contributions. Active maintainers should welcome such a move.
Retiring of other maintainers or regaining the status should require approval
of at least two active maintainers.

## Acknowledgements

Contributor roles and responsibilities were written based on [Kubernetes community membership]

[MAINTAINERS]: /MAINTAINERS
[contributor guide]: /CONTRIBUTING.md
[membership nomination]:https://github.com/etcd-io/etcd/issues/new?assignees=&labels=area%2Fcommunity&template=membership-request.yml 
[Kubernetes community membership]: https://github.com/kubernetes/community/blob/master/community-membership.md
[emeritus maintainers]: /README.md#etcd-emeritus-maintainers
[security disclosure and release process]: /security/README.md
[two-factor authentication]: https://docs.github.com/en/authentication/securing-your-account-with-two-factor-authentication-2fa/about-two-factor-authentication
