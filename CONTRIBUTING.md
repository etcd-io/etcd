# How to contribute

CoreOS projects are Apache 2.0 licensed and accept contributions via Github
pull requests.  This document outlines some of the conventions on commit
message formatting, contact points for developers and other resources to make
getting your contribution accepted.

# Certificate of Origin

By contributing to this project you agree to the Developer Certificate of
Origin (DCO). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the
contribution.

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```


# Email and chat

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
- Submit a pull request

Thanks for you contributions!

### Format of the commit message

We follow a rough convention for commit messages borrowed from Angularjs. This
is an example of a commit:

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

The first line is the subject and should not be longer than 70 characters, the
second line is always blank and other lines should be wrapped at 80 characters.
This allows the message to be easier to read on github as well as in various
git tools.

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

For more details see the [angularjs commit style
guide](https://docs.google.com/a/coreos.com/document/d/1QrDFcIiPjSLDn3EL15IJygNPiHORgU1_OOAqWjiDU5Y/edit#).
