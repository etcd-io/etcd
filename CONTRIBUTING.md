# How to contribute

etcd is open source, Apache 2.0 licensed and accepts contributions via Github pull requests.
This document outlines some of the conventions on commit message formatting, contact points for developers and other resources to make getting your contribution into etcd easier.

# Email and chat

For simplicity etcd discussions happen on coreos-dev and in #coreos-dev.
As the community grows we will move to a dedicated mailing list and IRC channel.

- Email: [coreos-dev](https://groups.google.com/forum/#!forum/coreos-dev) 
- IRC: #[coreos](irc://irc.freenode.org:6667/#coreos) IRC channel on freenode.org

## Getting Started

- Fork the repository on GitHub
- Read the README.md for build instructions

## Contribution flow

This is a rough outline of what a contributor's workflow looks like:

- Create a topic branch from where you want to base your work. This is usually master.
- Make commits of logical units.
- Make sure your commit messages are in the proper format, see below
- Push your changes to a topic branch in your fork of the repository.
- Submit a pull request to coreos/etcd

Thanks for you contributions!

### Format of the commit message

etcd follow a rough convention for commit messages borrowed from Angularjs.
This is an example of a commit:

```
    feat(scripts/test-cluster): add a cluster test command

    this uses tmux to setup a test cluster that you can easily kill and
    start for debugging.
```

To make it more formal it looks something like this:

```
<type>(<scope>): <subject>
<BLANK LINE>
<body>
<BLANK LINE>
<footer>
```

The first line is the subject and should not be longer than 70 characters, the second line is always blank and other lines should be wrapped at 80 characters.
This allows the message to be easier to read on github as well as in various git tools.

### Subject line

The subject line contains succinct description of the change.

### Allowed <type>
- feat (feature)
- fix (bug fix)
- docs (documentation)
- style (formatting, missing semi colons, …)
- refactor
- test (when adding missing tests)
- chore (maintain)

### Allowed <scope>

Scopes could be anything specifying place of the commit change. For example store, api, etc.

### More details on commits

For more details see the [angularjs commit style guide](https://docs.google.com/a/coreos.com/document/d/1QrDFcIiPjSLDn3EL15IJygNPiHORgU1_OOAqWjiDU5Y/edit#).
