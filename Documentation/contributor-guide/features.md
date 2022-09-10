# Features 

This document provides an overview of etcd features and general guidelines for adding and removing features. The project maintainers can override the guidelines per the need of the project following the project governance.

The etcd features fall into three stages, experimental, stable, and unsafe.

## Experimental

Any new feature should be added as an experimental feature. An experimental feature is characterized as below:
- Might be buggy due to lack of user testing. Enabling the feature may not work as expected.
- Support for such a feature may be dropped at any time without notice
- Disabled by default when added initially

### Minimum development requirements

Experimental feature related development work is expected to be high quality, similar to any development work for the project. A related PR should meet the following criteria,
- Associated with an issue providing a clear need for such a feature
- Any configuration flags related to the implementation of the feature are prefixed with `experimental` e.g. `--experimental-feature-name`
- Any variable names related to the implementation of the feature are prefixed with `Experimental` e.g. `ExperimentalFeatureName`
- Provided with unit tests
- Provided with e2e test coverage for at least the basic scenario
- Added a CHANGELOG entry
- Disabled by default
- Approved by at least two project maintainers

### Graduation to Stable

It is important that experimental features don't get stuck in that stage. After a release of an intiial enablement (e.g. etcd v3.5), an experimental feature should be revisited in the following release (e.g. etcd v3.6 or after six months if release takes longer) and see if it's ready for graduation to the stable stage in the future release (e.g. etcd v3.7 or v4.0). If it's ready per discussions with the project maintainers, then depending on the scope of the feature, at least the following enhancements should be consiered to be worked on by opening a PR(s) and new issue(s) for tracking as needed,
- Fix any open issues against the feature
- Add robust e2e tests
- Enhance logs for proper debugging
- Update metrics and benchmarks if needed
- Upgrade/downgrade tests specially if there are changes in API, Raft, WAL protos.
- Enable by default if appropriate
- Modify the feature documentation and add a CHANGELOG entry to reflect that `--experimental` prefix in the feature name is expected to be dropped in the next release e.g. `Expected to be graduated and renamed to --feature-name in the future release`
- Create a new issue for future graduation with reference to the PR discussed here. Add the release label of the upcoming release for tracking.

One release with above enhancements should give a good idea on the feature's stability. Unless there are open issues that can prevent feature gradution, the feature can move to the stable stage in the next release with a PR on feature rename, doc updates, and an entry in the CHANGELOG. All the related PRs and discussions should be approved by at least two project maintainers.

Patch releases should not be considered for graduation.

## Stable

A stable feature is characterized as below: 
- Supported as part of the supported releases of etcd.
- May be enabled by default.
- Discontinuation of support must follow the Feature Deprecation Process.

### Feature Deprecation Process

As the project evolves, a stable feature may sometimes need to be deprecated and removed. Such a situation should be handled using the steps below:
- Provide a deprecation message in the feature usage document before a planned release for feature deprecation. e.g. `To be deprecated in <release>`.
- Deprecate the feature in the planned release with an appropriate message as part of the feature usage documentation. e.g. `Deprecated`.
- Deprecated feature then be removed in the following release.

Overall, it can take two releases before a stable feature is completely removed. Patch releases should not be considered for deprecation.

## Unsafe

Unsafe features are rare and listed under the `Unsafe feature:` section in the etcd usage documentation. By default, they are disabled. They should be used with caution following documentation. Support for such features may be dropped at any time without notice.
