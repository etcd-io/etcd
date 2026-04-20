# Cherry-picking changes

Cherry-picking applies changes from a PR to a release branch.

## Recommended workflow (automated)

Cherry-picks are primarily handled using the **k8s-infra-cherrypick-robot**.

For example, to cherry-pick a PR onto `release-3.6`, comment on the PR with:

```bash
/cherry-pick release-3.6
```

The bot will create a cherry-pick PR against the target release branch.

---

## Fallback: manual cherry-pick script

This script can be used for manual cherry-picks in cases where the automated cherry-pick does not work.

## Setup

Set the `UPSTREAM_REMOTE` and `FORK_REMOTE` environment variables.
`UPSTREAM_REMOTE` should be set to git remote name of `github.com/etcd-io/etcd`,
and `FORK_REMOTE` should be set to the git remote name of the forked etcd
repo (`github.com/${github-username}/etcd`). Use `git remote -v` to
look up the git remote names. If etcd has not been forked, create
one on github.com and register it locally with `git remote add ...`.

```bash
export UPSTREAM_REMOTE=upstream
export FORK_REMOTE=origin
export GITHUB_USER=${github-username}
```

Next, install hub from <https://github.com/github/hub>

## Usage

To cherry pick PR 12345 onto release-3.6 and propose is as a PR, run:

```sh
./scripts/cherrypick.sh ${UPSTREAM_REMOTE}/release-3.6 12345
```

To cherry pick 12345 then 56789 and propose them togther as a single PR, run:

```bash
./scripts/cherrypick.sh ${UPSTREAM_REMOTE}/release-3.6 12345 56789
```
