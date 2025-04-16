# Bump etcd Version in Kubernetes

This guide will walk through the update of etcd in Kubernetes to a new version (`kubernetes/kubernetes` repository).

> Currently we bump etcd v3.5.x for K8s release-1.33 and lower versions, and we bump etcd v3.6.x for K8s release-1.34 and higher versions.

You can use this [issue](https://github.com/kubernetes/kubernetes/issues/131101) as a reference when updating the etcd version in Kubernetes.

Bumping the etcd version in Kubernetes consists of two steps.

* Bump etcd client SDK
* Bump etcd image

> The commented lines in this document signifies the line to be changed

## Bump etcd client SDK

> Reference: [link](https://github.com/kubernetes/kubernetes/pull/131103)

You can refer to the guide [here](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/vendor.md) under the **Adding or updating a dependency** section.

1. Get all the etcd modules used in Kubernetes.

    ```bash
    $ grep 'go.etcd.io/etcd/' go.mod | awk '{print $1}'
    go.etcd.io/etcd/api/v3
    go.etcd.io/etcd/client/pkg/v3
    go.etcd.io/etcd/client/v3
    go.etcd.io/etcd/client/v2
    go.etcd.io/etcd/pkg/v3
    go.etcd.io/etcd/raft/v3
    go.etcd.io/etcd/server/v3
    ```

2. For each module, in the root directory of the `kubernetes/kubernetes` repository, fetch the new version in `go.mod` using the following command (using `client/v3` as an example):

    ```bash
    hack/pin-dependency.sh go.etcd.io/etcd/client/v3 NEW_VERSION
    ```

3. Rebuild the `vendor` directory and update the `go.mod` files for all staging repositories using the command below. This automatically updates the licenses.

    ```bash
    hack/update-vendor.sh
    ```

4. Check if the new dependency requires newer versions of existing dependencies we have pinned. You can check this by:

    * Running `hack/lint-dependencies.sh` against your branch and against `master` and comparing the results.
    * Checking if any new `replace` directives were added to `go.mod` files of components inside the staging directory.

## Bump etcd image

### Build etcd image

> Reference: [link 1](https://github.com/kubernetes/kubernetes/pull/131105) [link 2](https://github.com/kubernetes/kubernetes/pull/131126)

1. In `build/dependencies.yaml`, update the `version` of `etcd-image` to the new version. Update `golang: etcd release version` if necessary.

    ```yaml
    - name: "etcd-image"
      # version: 3.5.17
      version: 3.5.21
      refPaths:
      - path: cluster/images/etcd/Makefile
        match: BUNDLED_ETCD_VERSIONS\?|
    ---
    - name: "golang: etcd release version"
      # version: 1.22.9
      version: 1.23.7 # https://github.com/etcd-io/etcd/blob/main/CHANGELOG/CHANGELOG-3.6.md
    ```

2. In `cluster/images/etcd/Makefile`, include the new version in `BUNDLED_ETCD_VERSIONS` and update the `LATEST_ETCD_VERSION` as well (the image tag will be generated from the `LATEST_ETCD_VERSION`). Update `GOLANG_VERSION` according to the version used to compile that release version (`"golang: etcd release version"` in step 1).

    ```Makefile
    # BUNDLED_ETCD_VERSIONS?=3.4.18 3.5.17
    BUNDLED_ETCD_VERSIONS?=3.4.18 3.5.21

    # LATEST_ETCD_VERSION?=3.5.17
    LATEST_ETCD_VERSION?=3.5.21

    # GOLANG_VERSION := 1.22.9
    GOLANG_VERSION := 1.23.7
    ```

3. In `cluster/images/etcd/migrate/options.go`, include the new version in the `supportedEtcdVersions` slice.

    ```go
    var (
    // supportedEtcdVersions = []string{"3.4.18", "3.5.17"}
    supportedEtcdVersions = []string{"3.4.18", "3.5.21"}
    )
    ```

### Publish etcd image

> Reference: [link](https://github.com/kubernetes/k8s.io/pull/7957)

1. When the previous step is merged, a post-commit job will run to build the image. You can find the newly built image in the [registry](https://gcr.io/k8s-staging-etcd/etcd).

2. Locate the newly built image and copy its SHA256 digest.

3. Inside the `kubernetes/k8s.io` repository, in `registry.k8s.io/images/k8s-staging-etcd/images.yaml`, create a new entry for the desired version and copy the SHA256 digest.

    ```yaml
    "sha256:b4a9e4a7e1cf08844c7c4db6a19cab380fbf0aad702b8c01e578e9543671b9f9": ["3.5.17-0"]
    # ADD:
    "sha256:d58c035df557080a27387d687092e3fc2b64c6d0e3162dc51453a115f847d121": ["3.5.21-0"]
    ```

### Update to use the new etcd image

> Reference: [link](https://github.com/kubernetes/kubernetes/pull/131144)

1. In `build/dependencies.yaml`, change the `version` of `etcd` to the new version.

    ```yaml
    # etcd
    - name: "etcd"
    # version: 3.5.17
    version: 3.5.21
    refPaths:
    - path: cluster/gce/manifests/etcd.manifest
    match: etcd_docker_tag|etcd_version
    ```

2. In `cluster/gce/manifests/etcd.manifest`, change the image tag to the new image tag and `TARGET_VERSION` to the new version.

    ```manifest
    // "image": "{{ pillar.get('etcd_docker_repository', 'registry.k8s.io/etcd') }}:{{ pillar.get('etcd_docker_tag', '3.5.17-0') }}",

    "image": "{{ pillar.get('etcd_docker_repository', 'registry.k8s.io/etcd') }}:{{ pillar.get('etcd_docker_tag', '3.5.21-0') }}",

    ---

    { "name": "TARGET_VERSION",
    // "value": "{{ pillar.get('etcd_version', '3.5.17') }}"
    "value": "{{ pillar.get('etcd_version', '3.5.21') }}"
    },
    ```

3. In `cluster/gce/upgrade-aliases.sh`, update the exports for `ETCD_IMAGE` to the new image tag and `ETCD_VERSION` to the new version.

    ```sh
    # export ETCD_IMAGE=3.5.17-0
    export ETCD_IMAGE=3.5.21-0
    # export ETCD_VERSION=3.5.17
    export ETCD_VERSION=3.5.21
    ```

4. In `cmd/kubeadm/app/constants/constants.go`, change the `DefaultEtcdVersion` to the new version. In the same file, update `SupportedEtcdVersion` accordingly.

    ```go
    // DefaultEtcdVersion = "3.5.17-0"
    DefaultEtcdVersion = "3.5.21-0"

    ---

    SupportedEtcdVersion = map[uint8]string{
    // 30: "3.5.17-0",
    // 31: "3.5.17-0",
    // 32: "3.5.17-0",
    // 33: "3.5.17-0",
    30: "3.5.21-0",
    31: "3.5.21-0",
    32: "3.5.21-0",
    33: "3.5.21-0",
    }
    ```

5. In `hack/lib/etcd.sh`, update the `ETCD_VERSION`.

    ```sh
    # ETCD_VERSION=${ETCD_VERSION:-3.5.17}
    ETCD_VERSION=${ETCD_VERSION:-3.5.21}
    ```

6. In `staging/src/k8s.io/sample-apiserver/artifacts/example/deployment.yaml`, update the etcd image used.

    ```yaml
    - name: etcd
    # image: gcr.io/etcd-development/etcd:v3.5.17
    image: gcr.io/etcd-development/etcd:v3.5.21
    ```

7. In `test/utils/image/manifest.go`, update the etcd image tag.

    ```go
    // configs[Etcd] = Config{list.GcEtcdRegistry, "etcd", "3.5.17-0"}
    configs[Etcd] = Config{list.GcEtcdRegistry, "etcd", "3.5.21-0"}
    ```
