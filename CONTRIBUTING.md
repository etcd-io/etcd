# How to contribute

etcd is Apache 2.0 licensed and accepts contributions via GitHub pull requests. This document outlines some of the conventions on commit message formatting, contact points for developers, and other resources to help get contributions into etcd.

# Email and chat

- Email: [etcd-dev](https://groups.google.com/forum/?hl=en#!forum/etcd-dev)
- IRC: #[etcd](irc://irc.freenode.org:6667/#etcd) IRC channel on freenode.org
- Slack: [#etcd](https://kubernetes.slack.com/messages/C3HD8ARJ5/details/)

## Getting started

- Fork the repository on GitHub
- Read the README.md for build instructions

## Reporting bugs and creating issues

Reporting bugs is one of the best ways to contribute. However, a good bug report has some very specific qualities, so please read over our short document on [reporting bugs](https://etcd.io/docs/latest/reporting_bugs) before submitting a bug report. This document might contain links to known issues, another good reason to take a look there before reporting a bug.

## Contribution flow

This is a rough outline of what a contributor's workflow looks like:

- Create a topic branch from where to base the contribution. This is usually main.
- Make commits of logical units.
- Make sure commit messages are in the proper format (see below).
- Push changes in a topic branch to a personal fork of the repository.
- Submit a pull request to etcd-io/etcd.
- The PR must receive a LGTM from two maintainers found in the MAINTAINERS file.

Thanks for contributing!

### Code style

The coding style suggested by the Golang community is used in etcd. See the [style doc](https://github.com/golang/go/wiki/CodeReviewComments) for details.

Please follow this style to make etcd easy to review, maintain and develop.

### Format of the commit message

We follow a rough convention for commit messages that is designed to answer two
questions: what changed and why. The subject line should feature the what and
the body of the commit should describe the why.

```
etcdserver: add grpc interceptor to log info on incoming requests

To improve debuggability of etcd v3. Added a grpc interceptor to log
info on incoming requests to etcd server. The log output includes
remote client info, request content (with value field redacted), request
handling latency, response size, etc. Uses zap logger if available,
otherwise uses capnslog.

Fixes #38
```

The format can be described more formally as follows:

```
<package>: <what changed>
<BLANK LINE>
<why this change was made>
<BLANK LINE>
<footer>
```

The first line is the subject and should be no longer than 70 characters, the second
line is always blank, and other lines should be wrapped at 80 characters. This allows
the message to be easier to read on GitHub as well as in various git tools.

### Pull request across multiple files and packages

If multiple files in a package are changed in a pull request for example:

```
etcdserver/config.go
etcdserver/corrupt.go
```

At the end of the review process if multiple commits exist for a single package they
should be squashed/rebased into a single commit before being merged.

```
etcdserver: <what changed>
[..]
```

If a pull request spans many packages these commits should be squashed/rebased into a single
commit using message with a more generic `*:` prefix.

```
*: <what changed>
[..]
```
