# PR management

## Purpose

Speed up PR management.

The `etcd` PRs are listed at https://github.com/etcd-io/etcd/pulls
A PR can have various labels, milestones, reviewers, etc. The detailed list of labels can be found at
https://github.com/kubernetes/kubernetes/labels

Following are a few example searches on PR for convenience:
* [Open PRS for milestone etcd-v3.6](https://github.com/etcd-io/etcd/pulls?utf8=%E2%9C%93&q=is%3Apr+is%3Aopen+milestone%3Aetcd-v3.6)
* [PRs under investigation](https://github.com/etcd-io/etcd/labels/Investigating)

## Scope

These guidelines serve as a primary document for managing PRs in `etcd`. Everyone is welcome to help manage PRs but the work and responsibilities discussed in this document are created with `etcd` maintainers and active contributors in mind.

## Ensure tests are run

The etcd project use Kubernetes Prow and GitHub Actions to run tests. To ensure all required tests run if a pull request is ready for testing and still has the `needs-ok-to-test` label then please comment on the pull request `/ok-to-test`.

## Handle inactive PRs
Poke PR owner if review comments are not addressed in 15 days. If the PR owner does not reply in 90 days, update the PR with a new commit if possible. If not, inactive PR should be closed after 180 days.

## Poke reviewer if needed

Reviewers are responsive in a timely fashion, but considering everyone is busy, give them some time after requesting a review if a quick response is not provided. If the response is not provided in 10 days, feel free to contact them via adding a comment in the PR or sending an email or message on Slack.

## Verify important labels are in place

Make sure that appropriate reviewers are added to the PR. Also, make sure that a milestone is identified. If any of these or other important labels are missing, add them. If a correct label cannot be decided, leave a comment for the maintainers to do so as needed.
