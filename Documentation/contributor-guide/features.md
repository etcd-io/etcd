# Features 

This document provides general guidelines for adding and removing etcd features. The etcd features fall into two stages, experimental and stable.

## Experimental

Any new feature should be added as an experimental feature. An experimental feature is characterized as below:
- Associated with an issue with a clear need for such a feature.
- Any configuration flags related to the implementation of the feature are prefixed with `experimental` e.g. `--experimental-feature-name`.
- Any variable names related to the implementation of the feature are prefixed with `Experimental` e.g. `ExperimentalFeatureName`.
- Might be buggy. Enabling the feature may not work as expected. 
- Support for such a feature may be dropped at any time without notice.
- Disabled by default when added initially.

### Graduation to Stable

It is important that experimental features don't get stuck in that stage.  An experimental feature should move to the stable stage by following the lifecycle steps below:
- A new feature added in the latest release and disabled by default e.g. etcd v3.5 or v4.0
- Given there are no open issues against the feature, it is ready for review to,
  - Enable by default in the next release e.g. etcd v3.6 or v4.1
  - Move to the stable stage in the following release e.g. etcd v3.7 or v4.2. If release cycle takes longer, one year time frame can be used for the review.

Patch releases should not be used for graduation.

## Stable

A stable feature is characterized as below: 
- Supported as part of the supported releases of etcd.
- May be enabled by default.
- Discontinuation of support must follow the Feature Deprecation Process.

### Feature Deprecation Process

As the project evolves, a stable feature may sometimes need to be deprecated and removed. Such a situation should be handled using the steps below:
- Provide a deprecation message in the feature usage document before a planned release for feature deprecation. e.g. `To be deprecated in <release>`.
- Deprecate the feature in the planned release with an appropriate message as part of the feature usage document. e.g. `Deprecated`.
- Deprecated feature then be removed in the following release.

Overall, it can take two releases before a stable feature is completely removed. Patch releases should not be used for deprecation.
