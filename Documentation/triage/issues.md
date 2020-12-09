---
title: Issue Triage Guidelines
weight: 8100
description: Managing incoming issues
---

## Purpose

Speed up issue management.

The `etcd` issues are listed at https://github.com/etcd-io/etcd/issues
and are identified with labels. For example, an issue that is identified
as a bug will eventually be set to label `area/bug `. New issues will
start out without any labels, but typically `etcd` maintainers and active contributors
add labels based on their findings. The detailed list of labels can be found at
https://github.com/kubernetes/kubernetes/labels

Following are few predetermined searches on issues for convenience:
* [Bugs](https://github.com/etcd-io/etcd/labels/area%2Fbug)
* [Help Wanted](https://github.com/etcd-io/etcd/labels/Help%20Wanted)
* [Longest untriaged issues](https://github.com/etcd-io/etcd/issues?utf8=%E2%9C%93&q=is%3Aopen+sort%3Aupdated-asc+)

## Scope

These guidelines serves as a primary document for triaging an incoming issues in
`etcd`. Everyone is welcome to help manage issues and PRs but the work and responsibilities discussed in this document are created with `etcd` maintainers and active contributors in mind.

## Validate if an issue is a bug

Validate if the issue is indeed a bug. If not, add a comment with findings and close trivial issue. For non-trivial issue, wait to hear back from issue reporter and see if there is any objection. If issue reporter does not reply in 30 days, close the issue. If the problem can not be reproduced or require more information, leave a comment for the issue reporter.

## Inactive issues

Issues that lack enough information from the issue reporter should be closed if issue reporter do not provide information in 60 days.

## Duplicate issues

If an issue is a duplicate, add a comment stating so along with a reference for the original issue and close it.

## Issues that don't belong to etcd

Sometime issues are reported that actually belongs to other projects that `etcd` use. For example, `grpc` or `golang` issues. Such issues should be addressed by asking reporter to open issues in appropriate other project. Close the issue unless a maintainer and issue reporter see a need to keep it open for tracking purpose.

## Verify important labels are in place

Make sure that issue has label on areas it belongs to, proper assignees are added and milestone is identified. If any of these labels are missing, add one. If labels can not be assigned due to limited privilege or correct label can not be decided, that’s fine, contact maintainers if needed.

## Poke issue owner if needed

If an issue owned by a developer has no PR created in 30 days, contact the issue owner and ask for a PR or to release ownership if needed.
