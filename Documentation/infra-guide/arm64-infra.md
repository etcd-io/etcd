# etcd arm64 test infrastructure

## Infrastructure summary

All etcd project pipelines run via github actions. The etcd project currently maintains dedicated infrastructure for running `arm64` continuous integration testing. This is required because currently github actions runner virtual machines are only offered as `x64`.

The infrastructure consists of two `c3.large.arm` bare metal servers kindly provided by [Equinix Metal](https://www.equinix.com/) via the [CNCF Community Infrastructure Lab](https://github.com/cncf/cluster/issues/227).

| Hostname                      | IP             | Operating System   | Region        |
|-------------------------------|----------------|--------------------|---------------|
| etcd-c3-large-arm64-runner-01 | 86.109.7.233   | Ubuntu 22.04.1 LTS | Washington DC |
| etcd-c3-large-arm64-runner-02 | 147.28.151.226 | Ubuntu 22.04.1 LTS | Washington DC |

## Infrastructure support

The etcd project aims to self manage and resolve issues with project infrastructure internally where possible, however if situations emerge where we need to engage support from Equinix Metal we can open an issue under the [CNCF Community Infrastructure Lab](https://github.com/cncf/cluster/issues) project. If the situation is urgent contact @vielmetti directly who can provide further assistance or escalation points.

## Granting infrastructure access

Etcd arm64 test infrastructure access is closely controlled to ensure the infrastructure is secure and protect the integrity of the etcd project.

Access to the infrastructure is defined by the infra admins table below:

| Name                      | Github         | K8s Slack          | Email              |
|---------------------------|----------------|--------------------|--------------------|
| Marek Siarkowicz          | @serathius     | @ Serathius        | Ref MAINTAINERS.md |
| Benjamin Wang             | @ahrtr         | @ Benjamin Wang    | Ref MAINTAINERS.md |
| Davanum Srinivas          | @dimns         | @ Dims             | davanum@gmail.com  |
| Chao Chen                 | @chaochn47     | @ Chao Chen        | chaochn@amazon.com |
| James Blair               | @jmhbnz        | @ James Blair      | etcd@jamma.life    |

Individuals in this table are granted access to the infrastructure in two ways:

### 1. Equinix metal web console access

An etcd project exists under the CNCF organisation in the Equinix Metal web console. The direct url to the etcd console is <https://console.equinix.com/projects/1b8c1eb7-983c-4b40-97e0-e317406e232e>.

When a new person is added to the infra admins table, an existing member or etcd maintainer should raise an issue in the [CNCF Community Infrastructure Labs](https://github.com/cncf/cluster/issues) to ensure they are granted web console access.

### 2. Server ssh access

Infra admins can ssh directly to the servers with a dedicated user account for each person, usernames are based on github handles for easy recognition in logs. These infra admins will be able to elevate to the `root` user when necessary via `sudo`.

Access to machines via ssh is strictly via individual ssh key based authentication, and is not permitted directly to the `root` user. Password authentication is never to be used for etcd infrastructure ssh authentication.

When a new member is added to the infra admins table, and existing member with ssh access should complete the following actions on all etcd servers:

- create the new user via `sudo adduser <username>`.
- add their public key to `/home/<username>/.ssh/authorized_keys` file. Note: Public keys are to be retrieved via github only, example: <https://github.com/jmhbnz.keys>.
- add the new user to machine sudoers file via `usermod -aG sudo <username>`.

## Revoking infrastructure access

When a member is removed from the infra admins table existing members must review servers and ensure their user access to etcd infrastructure is revoked by removing the members `/home/<username>/.ssh/authorized_keys` entries.

Note: When revoking access do not delete a user or their home directory from servers, as access may need to be reinstated in future.

## Regular access review

On a regular at least quarterly basis members of the infra admins team are responsible for verifying that no unneccessary infrastructure access exists by reviewing membership of the table above and existing server access.
