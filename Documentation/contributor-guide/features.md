# Features 

This document provides an overview of etcd features and general development guidelines for adding and deprecating them. The project maintainers can override these guidelines per the need of the project following the project governance.

## Overview

The etcd features fall into three stages, experimental, stable, and unsafe.

### Experimental

Any new feature is usually added as an experimental feature. An experimental feature is characterized as below:
- Might be buggy due to a lack of user testing. Enabling the feature may not work as expected.
- Disabled by default when added initially.
- Support for such a feature may be dropped at any time without notice
    - Feature-related issues may be given lower priorities.
    - It can be removed in the next minor or major release without following the feature deprecation policy unless it graduates to a stable future.

### Stable

A stable feature is characterized as below:
- Supported as part of the supported releases of etcd.
- May be enabled by default.
- Discontinuation of support must follow the feature deprecation policy.

### Unsafe

Unsafe features are rare and listed under the `Unsafe feature:` section in the etcd usage documentation. By default, they are disabled. They should be used with caution following documentation. An unsafe feature can be removed in the next minor or major release without following the feature deprecation policy.

## Development Guidelines

### Adding a new feature

Any new enhancements to the etcd are typically added as an experimental feature. The general development requirements are listed below. They can be somewhat flexible depending on the scope of the feature and review discussions and will evolve over time.
- Open an issue
    - It must provide a clear need for the proposed feature.
    - It should list development work items as checkboxes. There must be one work item towards future graduation to a stable future.
    - Label the issue with `type/feature` and `experimental`.
    - Keep the issue open for tracking purposes until a decision is made on graduation.
- Open a Pull Request (PR)
    - Provide unit tests. Integration tests are also recommended as possible.
    - Provide robust e2e test coverage. If the feature being added is complicated or quickly needed, maintainers can decide to go with e2e tests for basic coverage initially and have robust coverage added at a later time before the feature graduation to the stable feature.
    - Provide logs for proper debugging.
    - Provide metrics and benchmarks as needed.
    - The Feature should be disabled by default.
    - Any configuration flags related to the implementation of the feature must be prefixed with `experimental` e.g. `--experimental-feature-name`.
    - Add a CHANGELOG entry.
- At least two maintainers must approve feature requirements and related code changes.

### Graduating an Experimental feature to Stable

It is important that experimental features don't get stuck in that stage. They should be revisited and moved to the stable stage following the graduation steps as described here.

#### Locate graduation candidate
Decide if an experimental feature is ready for graduation to the stable stage.
- Find the issue that was used to enable the experimental feature initially. One way to find such issues is to search for issues with `type/feature` and `experimental` labels.
- Fix any known open issues against the feature.
- Make sure the feature was enabled for at least one previous release. Check the PR(s) reference from the issue to see when the feature-related code changes were merged.

#### Provide implementation
If an experimental feature is found ready for graduation to the stable stage, open a Pull Request (PR) with the following changes.
- Add robust e2e tests if not already provided.
- Add a new stable feature flag identical to the experimental feature flag but without the `--experimental` prefix.
- Deprecate the experimental feature following the [feature deprecation policy](#Deprecating-a-feature).
- Implementation must ensure that both the graduated and deprecated experimental feature flags work as expected. Note that both these flags will co-exist for the timeframe described in the feature deprecation policy.
- Enable the graduated feature by default if needed.

At least two maintainers must approve the work. Patch releases should not be considered for graduation.

### Deprecating a feature

#### Experimental
An experimental feature deprecates when it graduates to the stable stage.
- Add a deprecation message in the documentation of the experimental feature with a recommendation to use a related stable feature. e.g. `DEPRECATED. Use <feature-name> instead.`
- Add a `deprecated` label in the issue that was initially used to enable the experimental feature.

#### Stable
As the project evolves, a stable feature may sometimes need to be deprecated and removed. Such a situation should be handled using the steps below:
- Create an issue for tracking purposes.
- Add a deprecation message in the feature usage documentation before a planned release for feature deprecation. e.g. `To be deprecated in <release>.`. If a new feature replaces the `To be deprecated` feature, then also provide a message saying so. e.g. `Use <feature-name> instead.`.
- Deprecate the feature in the planned release with a message as part of the feature usage documentation. e.g. `DEPRECATED`. If a new feature replaces the deprecated feature, then also provide a message saying so. e.g. `DEPRECATED. Use <feature-name> instead.`.
- Add a `deprecated` label in the related issue.

Remove the deprecated feature in the following release. Close any related issue(s). At least two maintainers must approve the work. Patch releases should not be considered for deprecation.
