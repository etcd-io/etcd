# etcd Governance

## Principles

The etcd community adheres to the following principles:

- Open: etcd is open source.
- Welcoming and respectful: See [Code of Conduct](code-of-conduct.md).
- Transparent and accessible: Changes to the etcd code repository and CNCF related
activities (e.g. level, involvement, etc) are done in public.
- Merit: Ideas and contributions are accepted according to their technical merit for
the betterment of the project. For specific guidance on practical contribution steps
please see [CONTRIBUTING](./CONTRIBUTING.md) guide.

## Maintainers

Maintainers are first and foremost contributors that have shown they
are committed to the long term success of a project. Maintainership is about building
trust with the current maintainers of the project and being a person that they can
depend on to make decisions in the best interest of the project in a consistent manner.
The maintainers role can be a top-level or restricted to certain package/feature
depending upon their commitment in fulfilling the expected responsibilities as explained
below.

### Top-level maintainer

- Running the etcd release processes
- Ownership of test and debug infrastructure
- Triage GitHub issues to keep the issue count low (goal: under 100)
- Regularly review GitHub pull requests across all pkgs
- Providing cross pkg design review
- Monitor email aliases
- Participate when called upon in the [security disclosure and release process](security/README.md)
- General project maintenance

### Package/feature maintainer

- Ownership of test and debug failures in a pkg/feature
- Resolution of bugs triaged to a package/feature
- Regularly review pull requests to the pkg subsystem

### Nomination and retiring of maintainers

[Maintainers](./MAINTAINERS) file on the `main` branch reflects the latest
state of project maintainers. List of existing maintainers should be kept up to
date by existing maintainers to properly reflect community health and to gain
better understanding of recruiting need for new maintainers. Changes to list of
maintainers should be done by opening a pull request and CCing all the existing
maintainers.

Contributors who are interested in becoming a maintainer, if performing relevant
responsibilities, should discuss their interest with the existing maintainers.
New maintainers must be nominated by an existing maintainer and must be elected
by a supermajority of maintainers with a fallback on lazy consensus after three
business weeks inactive voting period and as long as two maintainers are on board.

Life priorities, interests, and passions can change. Maintainers can retire and
move to the [emeritus status](./README.md#etcd-emeritus-maintainers). If a
maintainer needs to step down, they should inform other maintainers, if possible,
help find someone to pick up the related work. At the very least, ensure the
related work can be continued. Afterward they can remove themselves from list of
existing maintainers.

If a maintainer is not been performing their duties for period of 12 months,
they can be removed by other maintainers. In that case inactive maintainer will
be first notified via an email. If situation doesn't improve, they will be
removed. If an emeritus maintainer wants to regain an active role, they can do
so by renewing their contributions. Active maintainers should welcome such a move.
Retiring of other maintainers or regaining the status should require approval
of at least two active maintainers.

## Reviewers

[Reviewers](./MAINTAINERS) are contributors who have demonstrated greater skill in
reviewing the code contribution from other contributors. Their LGTM counts towards
merging a code change into the project. A reviewer is generally on the ladder towards
maintainership. New reviewers must be nominated by an existing maintainer and must be
elected by a supermajority of maintainers with a fallback on lazy consensus after three
business weeks inactive voting period and as long as two maintainers are on board.
Reviewers can be removed by a supermajority of the  maintainers or can resign by notifying
the maintainers.

## Decision making process

Decisions are built on consensus between maintainers publicly. Proposals and ideas
can either be submitted for agreement via a GitHub issue or PR, or by sending an email
to `etcd-maintainers@googlegroups.com`.

## Conflict resolution

In general, we prefer that technical issues and maintainer membership are amicably
worked out between the persons involved. However, any technical dispute that has
reached an impasse with a subset of the community, any contributor may open a GitHub
issue or PR or send an email to `etcd-maintainers@googlegroups.com`. If the
maintainers themselves cannot decide an issue, the issue will be resolved by a
supermajority of the maintainers with a fallback on lazy consensus after three business
weeks inactive voting period and as long as two maintainers are on board.

## Changes in Governance

Changes in project governance could be initiated by opening a GitHub PR.
