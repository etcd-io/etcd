# How to contribute

etcd is Apache 2.0 licensed and accepts contributions via Github pull requests. This document outlines some of the conventions on commit message formatting, contact points for developers and other resources to make getting your contribution into etcd easier.

# Email and chat

- Email: [etcd-dev](https://groups.google.com/forum/?hl=en#!forum/etcd-dev)
- IRC: #[coreos](irc://irc.freenode.org:6667/#coreos) IRC channel on freenode.org

## Getting started

- Fork the repository on GitHub
- Read the README.md for build instructions

## Contribution flow

This is a rough outline of what a contributor's workflow looks like:

- Create a topic branch from where you want to base your work. This is usually master.
- Make commits of logical units.
- Make sure your commit messages are in the proper format (see below).
- Push your changes to a topic branch in your fork of the repository.
- Submit a pull request to coreos/etcd.

Thanks for your contributions!

### Code style

The coding style suggested by the Golang community is used in etcd. See [style doc](https://code.google.com/p/go-wiki/wiki/Style) for details.

Please follow this style to make etcd easy to review, maintain and develop.

### Format of the commit message

etcd follows a rough convention for commit messages borrowed from Angularjs. This is an example of a commit:

```
    feat(scripts/test-cluster): add a cluster test command

    this uses tmux to setup a test cluster that you can easily kill and
    start for debugging.
```

The format can be more formally described as follows:

```
<type>(<scope>): <subject>
<BLANK LINE>
<body>
<BLANK LINE>
<footer>
```

The first line is the subject and should be no longer than 70 characters, the second line is always blank, and other lines should be wrapped at 80 characters.  This allows the message to be easier to read on github as well as
in various git tools.

### Subject line

The subject line contains a succinct description of the change.

### Allowed <type>s
- feat (feature)
- fix (bug fix)
- docs (documentation)
- style (formatting, missing semi colons, …)
- refactor
- test (when adding missing tests)
- chore (maintain)

### Allowed <scope>s

Scopes can be anything specifying the place of the commit change within the repository. For example, "store", "API", etc.

### More details on commits

For more details see the [angularjs commit style guide](https://docs.google.com/a/coreos.com/document/d/1QrDFcIiPjSLDn3EL15IJygNPiHORgU1_OOAqWjiDU5Y/edit#).
