# How to contribute

etcd is Apache 2.0 licensed and accepts contributions via GitHub pull requests.
This document outlines basics of contributing to etcd.

This is a rough outline of what a contributor's workflow looks like:
* [Find something to work on](#Find-something-to-work-on)
* [Setup development environment](#Setup-development-environment)
* [Implement your change](#Implement-your-change)
* [Commit your change](#Commit-your-change)
* [Create a pull request](#Create-a-pull-request)
* [Get your pull request reviewed](#Get-your-pull-request-reviewed)

If you have any questions about, please reach out using one of the methods listed in [contact].

[contact]: ./README.md#Contact

## Learn more about etcd

Before making a change please look through resources below to learn more about etcd and tools used for development.

* Please learn about [Git](https://github.com/git-guides) version control system used in etcd.
* Read the [etcd learning resources](https://etcd.io/docs/v3.5/learning/)
* Read the [etcd community membership](/Documentation/contributor-guide/community-membership.md)
* Watch [etcd deep dive](https://www.youtube.com/watch?v=D2pm6ufIt98&t=927s)
* Watch [etcd code walk through](https://www.youtube.com/watch?v=H3XaSF6wF7w)

## Find something to work on

All the work in etcd project is tracked in [github issue tracker].
Issues should be properly labeled making it easy to find something for you.

Depending on your interest and experience you should check different labels:
* If you are just starting, check issues labeled with [good first issue].
* When you feel more conformable in your contributions, checkout [help wanted].
* Advanced contributors can try to help with issues labeled [priority/important] covering most relevant work at the time.

If any of aforementioned labels don't have unassigned issues, please [contact] one of the [maintainers] asking to triage more issues.

[github issue tracker]: https://github.com/etcd-io/etcd/issues
[good first issue]: https://github.com/search?type=issues&q=org%3Aetcd-io+state%3Aopen++label%3A%22good+first+issue%22
[help wanted]: https://github.com/search?type=issues&q=org%3Aetcd-io+state%3Aopen++label%3A%22help+wanted%22
[maintainers]: https://github.com/etcd-io/etcd/blob/main/MAINTAINERS
[priority/important]: https://github.com/search?type=issues&q=org%3Aetcd-io+state%3Aopen++label%3A%22priority%2Fimportant%22

## Setup development environment

The etcd project supports two options for development:

 1. Manually setup local environment.
 2. Automatically setup [devcontainer](https://containers.dev).

For both options the only supported architecture is `linux-amd64`. Bug reports for other environments will generally be ignored. Supporting new environments requires introduction of proper tests and mainter support that is currently lacking in the etcd project.

If you would like etcd to support your preferred environment you can [file an issue].

### Option 1 - Manually setup local environment

This is the original etcd development environment, is most supported and is backwards compatible for development of older etcd versions.

Follow the steps below to setup the environment:

- [Clone the repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/cloning-a-repository)
- Install Go by following [installation](https://go.dev/doc/install). Please check minimal go version in [go.mod file](./go.mod#L3).
- Install build tools (`make`):
  - For debian based distributions you can run `sudo apt-get install build-essential`
- Verify that everything is installed by running `make build`

Note: `make build` runs with `-v`. Other build flags can be added through env `GO_BUILD_FLAGS`, **if required**. Eg.,
```console
GO_BUILD_FLAGS="-buildmode=pie" make build
```

### Option 2 - Automatically setup devcontainer

This is a more recently added environmnent that aims to make it faster for new contributors to get started with etcd. This option is supported for etcd versions 3.6 onwards.

This option can be [used locally](https://code.visualstudio.com/docs/devcontainers/tutorial) on a system running Visual Studio Code and Docker, or in a remote cloud based [Codespaces](https://github.com/features/codespaces) environment.

To get started, create a codespace for this repository by clicking this 👇

[![Open in GitHub Codespaces](https://github.com/codespaces/badge.svg)](https://github.com/codespaces/new?hide_repo_select=true&ref=main&repo=11225014)

A codespace will open in a web-based version of Visual Studio Code. The [dev container](.devcontainer/devcontainer.json) is fully configured with software needed for this project.

**Note**: Dev containers is an open spec which is supported by [GitHub Codespaces](https://github.com/codespaces) and [other tools](https://containers.dev/supporting).

[file an issue]: https://github.com/etcd-io/etcd/issues/new/choose

## Implement your change

etcd code should follow coding style suggested by the Golang community.
See the [style doc](https://github.com/golang/go/wiki/CodeReviewComments) for details.

Please ensure that your change passes static analysis (requires [golangci-lint](https://golangci-lint.run/usage/install/)):
- `make verify` to verify if all checks pass.
- `make verify-*` to verify a single check, for example `make verify-bom` to verify if bill-of-materials.json file is up-to-date.
- `make fix` to fix all checks.
- `make fix-*` to fix a single checks, for example `make fix-bom` to update bill-of-materials.json.

Please ensure that your change passes tests.
- `make test-unit` to run unit tests.
- `make test-integration` to run integration tests.
- `make test-e2e` to run e2e tests.

All changes are expected to come with unit test.
All new features are expected to have either e2e or integration tests.

## Commit your change

etcd follows a rough convention for commit messages:
* First line:
  * Should start name of package (for example `etcdserver`, `etcdctl`) followed by `:` character.
  * Describe the `what` behind the change
* Optionally author might provide the `why` behind the change in the main commit message body.
* Last line should be `Signed-off-by: firstname lastname <email@example.com>` (can be automatically generate by providing `--signoff` to git commit command).

Example of commit message:
```
etcdserver: add grpc interceptor to log info on incoming requests

To improve debuggability of etcd v3. Added a grpc interceptor to log
info on incoming requests to etcd server. The log output includes
remote client info, request content (with value field redacted), request
handling latency, response size, etc. Uses zap logger if available,
otherwise uses capnslog.

Signed-off-by: FirstName LastName <github@github.com>
```

## Create a pull request

Please follow [making a pull request](https://docs.github.com/en/get-started/quickstart/contributing-to-projects#making-a-pull-request) guide.

If you are still working on the pull request, you can convert it to draft by clicking `Convert to draft` link just below list of reviewers.

Multiple small PRs are preferred over single large ones (>500 lines of code).

## Get your pull request reviewed

Before requesting review please ensure that all GitHub checks were successful.
It might happen that some unrelated tests on your PR are failing, due to their flakiness.
In such cases please [file an issue] to deflake the problematic test and ask one of [maintainers] to rerun the tests.

If all checks were successful feel free to reach out for review from people that were involved in the original discussion or [maintainers].
Depending on complexity of the PR it might require between 1 and 2 maintainers to approve your change before merging.

Thanks for contributing!
