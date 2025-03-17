# Features 

This document provides an overview of etcd features and general development guidelines for adding and deprecating them. The project maintainers can override these guidelines per the need of the project following the project governance.

## Overview

The etcd features fall into three stages: Alpha, Beta, and GA.

### Alpha

Any new feature is usually added as an Alpha feature. An Alpha feature is characterized as below:
- Might be buggy due to a lack of user testing. Enabling the feature may not work as expected.
- Disabled by default.
- Support for such a feature may be dropped at any time without notice
    - Feature-related issues may be given lower priorities.
    - It can be removed in the next minor or major release without following the feature deprecation policy unless it graduates to a more stable stage.

### Beta

A Beta feature is characterized as below:
- Supported as part of the supported releases of etcd.
- Enabled by default.
- Discontinuation of support must follow the feature deprecation policy.

### GA

A GA feature is characterized as below:
- Supported as part of the supported releases of etcd.
- Always enabled; you cannot disable it. The corresponding feature gate is no longer needed.
- Discontinuation of support must follow the feature deprecation policy.

## Development Guidelines

### Adding a new feature

Any new enhancements to the etcd are typically added as an Alpha feature. 

etcd follows the Kubernetes [KEP process](https://github.com/kubernetes/enhancements/blob/master/keps/sig-architecture/0000-kep-process/README.md) for new enhancements. The general development requirements are listed below. They can be somewhat flexible depending on the scope of the feature and review discussions and will evolve over time.
- Open a [KEP](https://github.com/kubernetes/enhancements/issues) issue
    - It must provide a clear need for the proposed feature.
    - It should list development work items as checkboxes. There must be one work item towards future graduation to Beta.
    - Label the issue with `/sig etcd`.
    - Keep the issue open for tracking purposes until a decision is made on graduation.
- Open a [KEP](https://github.com/kubernetes/enhancements) Pull Request (PR). 
    - The KEP template can be simplified for etcd.
    - It must provide clear graduation criteria for each stage.
    - The KEP doc should reside in [keps/sig-etcd](https://github.com/kubernetes/enhancements/tree/master/keps/sig-etcd/)
- Open Pull Requests (PRs) in [etcd](https://github.com/etcd-io/etcd)
    - Provide unit tests. Integration tests are also recommended as possible.
    - Provide robust e2e test coverage. If the feature being added is complicated or quickly needed, maintainers can decide to go with e2e tests for basic coverage initially and have robust coverage added at a later time before the feature graduation to the stable feature.
    - Provide logs for proper debugging.
    - Provide metrics and benchmarks as needed.
    - Add an Alpha [feature gate](https://etcd.io/docs/v3.6/feature-gates/).
    - Any code changes or configuration flags related to the implementation of the feature must be gated with the feature gate e.g. `if cfg.ServerFeatureGate.Enabled(features.FeatureName)`.
    - Add a CHANGELOG entry.
- At least two maintainers must approve the KEP and related code changes.

### Graduating a feature to the next stage

It is important that features don't get stuck in one stage. They should be revisited and moved to the next stage once they meet the graduation criteria listed in the KEP. A feature should stay at one stage for at least one release before being promoted.

#### Provide implementation

If a feature is found ready for graduation to the next stage, open a Pull Request (PR) with the following changes.
- Update the feature `PreRelease` stage in `server/features/etcd_features.go`.
- Update the status in the original KEP issue.

At least two maintainers must approve the work. Patch releases should not be considered for graduation.

### Deprecating a feature

#### Alpha
Alpha features can be removed without going through the deprecation process.
- Remove the feature gate in `server/features/etcd_features.go`, and clean up all relevant code.
- Close the original KEP issue with reasons to drop the feature.

#### Beta and GA
As the project evolves, a Beta/GA feature may sometimes need to be deprecated and removed. Such a situation should be handled using the steps below:

- A Beta/GA feature can only be deprecated after at least 2 minor or major releases.
- Update original KEP issue if it has not been closed or create a new etcd issue with reasons and steps to deprecate the feature.
- Add the feature deprecation documentation in the release notes and feature gates documentation of the next minor/major release.
- In the next minor/major release, set the feature gate to `{Default: false, PreRelease: featuregate.Deprecated, LockedToDefault: false}` in `server/features/etcd_features.go`. Deprecated feature gates must respond with a warning when used.
    - If the feature has GAed, and the original gated codes has been cleaned up, add the disablement codes back with the feature gate.
- In the minor/major release after the next, set the feature gate to `{Default: false, PreRelease: featuregate.Deprecated, LockedToDefault: true}` in `server/features/etcd_features.go`, and start cleaning the code.

At least two maintainers must approve the work. Patch releases should not be considered for deprecation.
