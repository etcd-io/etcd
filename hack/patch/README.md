# hack/cherrypick.sh

Handles cherry-picks of PR(s) from etcd master to a stable etcd release branch automatically.

## Setup

Set the `UPSTREAM_REMOTE` and `FORK_REMOTE` environment variables.
`UPSTREAM_REMOTE` should be set to git remote name of `github.com/coreos/etcd`,
and `FORK_REMOTE` should be set to the git remote name of your fork of the etcd
repo (`github.com/${github-username}/etcd`). Use `git remotes -v` if you need to
look up your git remote names. If you don't already have a fork of etcd create
one on github.com and register it locally with `git remote add ...`.


```
export UPSTREAM_REMOTE=origin
export FORK_REMOTE=${github-username}
```

Next, install hub from https://github.com/github/hub

## Usage

To cherry pick PR 12345 onto release-2.22 and propose is as a PR, run:

```sh
hack/cherrypick.sh upstream/release-2.2 12345
```

To cherry pick 12345 then 56789 and propose them togther as a single PR, run:

```
hack/cherrypick.sh upstream/release-2.2 12345 56789
```


