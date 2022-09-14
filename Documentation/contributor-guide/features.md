# Features 

This document provides an overview of etcd features and general development guidelines for adding and deprecating them. The project maintainers can override these guidelines per the need of the project following the project governance.

## Overview

The etcd features fall into three stages, experimental, stable, and unsafe.

### Experimental

Any new feature is usually added as an experimental feature. An experimental feature is characterized as below:
- Might be buggy due to a lack of user testing. Enabling the feature may not work as expected.
- Disabled by default when added initially.
- Support for such a feature may be dropped at any time without notice
    - Feature related issues may be given lower priorities.
    - It can be removed in the next minor or major release without following the feature deprecation policy unless it graduates to the stable future.

### Stable

A stable feature is characterized as below:
- Supported as part of the supported releases of etcd.
- May be enabled by default.
- Discontinuation of support must follow the feature deprecation policy.

### Unsafe

Unsafe features are rare and listed under the `Unsafe feature:` section in the etcd usage documentation. By default, they are disabled. They should be used with caution following documentation. An unsafe feature can be removed in the next minor or major release without following feature deprecation policy.

## Development Guidelines

### Adding a new feature

Any new enhancements to the etcd are typically added as an experimental feature. The general development requirements are listed below. They can be somewhat flexible depending on the scope of the feature and review discussions, and will evolve over time.
- Open an issue
    - It must provide a clear need for the proposed feature.
    - It should list development work items as checkboxes. There must be one work item towards future graduation to the stable future.
    - Keep the issue open for tracking purpose until a decision is made on graduation.
- Open a Pull Request (PR)
    - Provide unit tests. Integreation tests are also recommended as possible.
    - Provide robust e2e test coverage. If the feature being added is complicated or quickly needed, maintainers can decide to go with e2e tests for basic coverage initially and have robust coverage added at the later time before feature graduation to the stable feature.
    - Provide logs for proper debugging.
    - Provide metrics and benchmarks as needed.
    - The Feature should be disabled by default.
    - Any configuration flags related to the implementation of the feature must be prefixed with `experimental` e.g. `--experimental-feature-name`.
    - Add a CHANGELOG entry.
- At least two maintainers must approve feature requirements and related code changes.

### Graduating an Experimental feature to Stable (TODO - spzala)

### Deprecating a feature (TODO - spzala)
