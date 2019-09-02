# ./hack/patch/cherrypick.sh

Handles cherry-picks of PR(s) from etcd master to a stable etcd release branch automatically.

## Setup

Set the `UPSTREAM_REMOTE` and `FORK_REMOTE` environment variables.
`UPSTREAM_REMOTE` should be set to git remote name of `github.com/etcd-io/etcd`,
and `FORK_REMOTE` should be set to the git remote name of the forked etcd
repo (`github.com/${github-username}/etcd`). Use `git remotes -v` to
look up the git remote names. If etcd has not been forked, create
one on github.com and register it locally with `git remote add ...`.


```
export UPSTREAM_REMOTE=origin
export FORK_REMOTE=${github-username}
export GITHUB_USER=${github-username}
```

Next, install hub from https://github.com/github/hub

## Usage

To cherry pick PR 12345 onto release-3.2 and propose is as a PR, run:

```sh
./hack/patch/cherrypick.sh ${UPSTREAM_REMOTE}/release-3.2 12345
```

To cherry pick 12345 then 56789 and propose them togther as a single PR, run:

```
./hack/patch/cherrypick.sh ${UPSTREAM_REMOTE}/release-3.2 12345 56789
```


