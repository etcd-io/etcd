# Issue triage guidelines

## Purpose

Speed up issue management.

The `etcd` issues are listed at <https://github.com/etcd-io/etcd/issues> and are identified with labels. For example, an issue that is identified as a bug will be set to label `type/bug`.

The etcd project uses labels to indicate common attributes such as `area`, `type` and `priority` of incoming issues.

New issues will often start out without any labels, but typically `etcd` maintainers, reviewers and members will add labels by following these triage guidelines. The detailed list of labels can be found at <https://github.com/etcd-io/etcd/labels>.

## Scope

This document serves as the primary guidelines for triaging incoming issues in `etcd`.

All contributors are encouraged and welcome to help manage issues which will help reduce burden on project maintainers, though the work and responsibilities discussed in this document are created with `etcd` project reviewers and members in mind as these individuals will have triage access to the etcd project which is a requirement for actions like applying labels or closing issues.

Refer to [etcd community membership](https://github.com/etcd-io/etcd/blob/main/Documentation/contributor-guide/community-membership.md) for guidance on becoming and etcd project member or reviewer.

## Step 1 - Find an issue to triage

To get started you can use the following recommended issue searches to identify issues that are in need of triage:

* [Issues that have no labels](https://github.com/etcd-io/etcd/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated+no%3Alabel)
* [Issues created recently](https://github.com/etcd-io/etcd/issues?q=is%3Aissue+is%3Aopen+)
* [Issues not assigned but linked pr](https://github.com/etcd-io/etcd/issues?q=is%3Aopen+is%3Aissue+no%3Aassignee+linked%3Apr)
* [Issues with no comments](https://github.com/etcd-io/etcd/issues?q=is%3Aopen+is%3Aissue+comments%3A0+)
* [Issues with help wanted](https://github.com/etcd-io/etcd/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22+)

## Step 2 - Check the issue is valid

Before we start adding labels or trying to work out a priority, our first triage step needs to be working out if the issue actually belongs to the etcd project and is not a duplicate.

### Issues that don't belong to etcd

Sometime issues are reported that actually belongs to other projects that `etcd` use. For example, `grpc` or `golang` issues. Such issues should be addressed by asking reporter to open issues in appropriate other project.

These issues can generally be closed unless a maintainer and issue reporter see a need to keep it open for tracking purpose. If you have triage permissions please close it, alternatively mention the @etcd-io/members group to request a member with triage access close the issue.

### Duplicate issues

If an issue is a duplicate, add a comment stating so along with a reference for the original issue and if you have triage permissions please close it, alternatively mention the @etcd-io/members group to request a member with triage access close the issue.

## Step 3 - Apply the appropriate type label

Adding a `type` label to an issue helps create visibility on the health of the project and helps contributors identify potential priorities, i.e. addressing existing bugs or test flakes before implementing new features.

### Support requests

Some people mistakenly use GitHub issues to file support requests. Usually they are asking for help operating or configuring some aspect of etcd. In general we prefer that support requests are raised as [discussions](https://github.com/etcd-io/etcd/discussions), however if you identify that an issue is a support request please add the `type/support` or `type/question` label.

When responding to these requests please keep in mind the etcd [supported versions policy](https://etcd.io/docs/v3.6/op-guide/versioning/#service-versioning-and-supported-versions). If the request relates to an unsupported version and will require significant effort please instead kindly make the issue creator aware of the etcd support policy, encourage they update to a supported release, then close the issue, refer to [this example](https://github.com/etcd-io/etcd/issues/15796).

### Bug reports

If an issue has been raised as a bug it should already have the `type/bug` label, however if this is missing for an issue you determine to be a bug please add the label manually.

The next step is to validate if the issue is indeed a bug. If not, add a comment with findings and close trivial issue. For non-trivial issue, wait to hear back from issue reporter and see if there is any objection. If issue reporter does not reply in 30 days, close the issue.

If the problem can not be reproduced or requires more information, leave a comment for the issue reporter as soon as possible while the issue will be fresh for the issue reporter.

### Feature requests

New feature requests should be created via the etcd feature request template and in theory already have the `type/feature` label, however if this is missing for an issue you determine to be a feature please add the label manually.

### Test flakes

Test flakes are a specific type of bug that the etcd project tracks seperately as these are a priority to address. These should be created via the test flake template and in theory already have the `type/flake` label, however if this is missing for an issue you determine to be related to a flaking test please add the label manually.

## Step 4 - Define the areas impacted

Adding an `area` label to an issue helps create visibility on which areas of the etcd project require attention and helps contributors find issues to work on relating to their particular skills or knowledge of the etcd codebase.

If an issue crosses multiple domains please add additional `area` labels to reflect that.

Below is a brief summary of the area labels in active use by the etcd project along with any notes on their use:

| Label                   | Notes                                                         |
| ---                     | ---                                                           |
| area/external           | Tracking label for issues raised that are external to etcd.   |
| area/community          |                                                               |
| area/raft               |                                                               |
| area/clientv3           |                                                               |
| area/performance        |                                                               |
| area/security           |                                                               |
| area/tls                |                                                               |
| area/auth               |                                                               |
| area/etcdctl            |                                                               |
| area/etcdutl            |                                                               |
| area/contrib            | Not to be confused with `area/community` this label is specifically used for issues relating to community maintained scripts or files in the `contrib/` directory which aren't part of the core etcd project. |
| area/documentation      |                                                               |
| area/tooling            | Generally used in relation to the third party / external utilities or tools that are used in various stages of the etcd build, test or release process, for example tooling to create sboms.          |
| area/testing            |                                                               |
| area/robustness-testing |                                                               |

## Step 5 - Prioritise the issue

Placeholder.

## Step 6 - Support new contributors

As part of the `etcd` triage process once the `kind` and `area` have been determined, please consider if the issue would be suitable for a less experienced contributor. The `good first issue` label is a subset of the `help wanted` label, indicating that members have committed to providing extra assistance for new contributors. All `good first issue` items also have the `help wanted` label.

### Help wanted

Items marked with the `help wanted` label need to ensure that they meet these criteria:

* **Low Barrier to Entry** - It should be easy for new contributors.

* **Clear** - The task is agreed upon and does not require further discussions in the community.

* **Goldilocks priority** - The priority should not be so high that a core contributor should do it, but not too low that it isn’t useful enough for a core contributor to spend time reviewing it, answering questions, helping get it into a release, etc.

### Good first issue

Items marked with `good first issue` are intended for first-time contributors. It indicates that members will keep an eye out for these pull requests and shepherd it through our processes.

New contributors should not be left to find an approver, ping for reviews, decipher test commands, or identify that their build failed due to a flake. It is important to make new contributors feel welcome and valued. We should assure them that they will have an extra level of help with their first contribution.

After a contributor has successfully completed one or two `good first issue` items, they should be ready to move on to `help wanted` items.

* **No Barrier to Entry** - The task is something that a new contributor can tackle without advanced setup or domain knowledge.

* **Solution Explained** - The recommended solution is clearly described in the issue.

* **Gives Examples** - Link to examples of similar implementations so new contributors have a reference guide for their changes.

* **Identifies Relevant Code** - The relevant code and tests to be changed should be linked in the issue.

* **Ready to Test** - There should be existing tests that can be modified, or existing test cases fit to be copied. If the area of code doesn’t have tests, before labeling the issue, add a test fixture. This prep often makes a great help wanted task!

## Step 7 - Follow up

Once initial triage has been completed, issues need to be re-evaluated over time to ensure they don't become stale incorrectly.

### Track important issues

If an issue is at risk of being closed by stale bot in future, but is an important issuefor the etcd project, then please apply the `stage/tracked` label and remove any `stale` labels that exist. This will ensure the project does not lose sight of the issue.

### Close incomplete issues

Issues that lack enough information from the issue reporter should be closed if issue reporter do not provide information in 30 days. Issues can always be re-opened at a later date if new information is provided.

### Check for incomplete work

If an issue owned by a developer has no pull request created in 30 days, contact the issue owner and kindly ask about the status of their work, or to release ownership on the issue if needed.
