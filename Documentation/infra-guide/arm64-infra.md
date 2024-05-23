# etcd arm64 test infrastructure

The infrastructure to build for arm64 is provided by [Equinix Metal](https://www.equinix.com/) via the [CNCF Community Infrastructure Lab](https://github.com/cncf/cluster/issues).

Previously, several maintainers were responsible for managing two bare-metal machines with a self-hosted runner installed. This was a manual process, and side effects could be left over from previous builds.

As part of a joint program between Ampere and the CNCF, [actuated.dev](https://actuated.dev) is providing managed Arm64 builds.

To use the new infrastructure, add the following to your workflow:

```yaml
runs-on: actuated-arm64-8cpu-8gb
```

The vCPUs and RAM are customizable, i.e. `actuated-arm64-8cpu-16gb` or `actuated-arm64-8cpu-32gb`.

For urgent support, contact @alexellis or the [actuated team](https://actuated.dev).
