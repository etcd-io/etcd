# etcd Prow Documentation

Prow is a Kubernetes based CI/CD system developed to serve the Kubernetes community. 
-- https://github.com/kubernetes-sigs/prow

etcd uses Prow to perform various CI tests to maintain the guarantees that etcd promises.

## List of Workflows

### Presubmits

Presubmits are run after a PR is created, given an `ok-to-test`, and before the PR is merged.

#### `main`

| Name | Link | Description |
| --- | --- | --- |
| pull-etcd-build | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L4-L28) |
| pull-etcd-unit-test-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L30-L58) |
| pull-etcd-unit-test-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L60-L89) |
| pull-etcd-unit-test-386 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L91-L119) |
| pull-etcd-verify | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L121-L158) |
| pull-etcd-govulncheck | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L160-L187) |
| pull-etcd-e2e-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L189-L217) |
| pull-etcd-e2e-386 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L219-L246) |
| pull-etcd-e2e-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L248-L277) |
| pull-etcd-integration-1-cpu-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L279-L309) |
| pull-etcd-integration-1-cpu-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L311-L343) |
| pull-etcd-integration-2-cpu-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L345-L375) |
| pull-etcd-integration-2-cpu-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L377-L409) |
| pull-etcd-integration-4-cpu-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L411-L441) |
| pull-etcd-integration-4-cpu-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L443-L475) |
| pull-etcd-robustness-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L443-L475) |
| pull-etcd-robustness-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L555-L602) |
| pull-etcd-release-tests | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L604-L639) |
| pull-etcd-contrib-mixin | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L641-L669) |
| pull-etcd-fuzzing-v3rpc | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L671-L704) |
| pull-etcd-grpcproxy-integration-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L706-L735) |
| pull-etcd-grpcproxy-e2e-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L737-L766) |
| pull-etcd-grpcproxy-integration-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L768-L798) |
| pull-etcd-grpcproxy-e2e-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L768-L798) |
| pull-etcd-coverage-report | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L832-L860) |
| pull-etcd-markdown-lint | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L862-L889)

#### `release-3.6`

| Name | Link | Description |
| --- | --- | --- |
| pull-etcd-build | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L4-L28) |
| pull-etcd-unit-test-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L30-L58) |
| pull-etcd-unit-test-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L60-L89) |
| pull-etcd-unit-test-386 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L91-L119) |
| pull-etcd-verify | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L121-L158) |
| pull-etcd-govulncheck | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L160-L187) |
| pull-etcd-e2e-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L189-L217) |
| pull-etcd-e2e-386 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L219-L246) |
| pull-etcd-e2e-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L248-L277) |
| pull-etcd-integration-1-cpu-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L279-L309) |
| pull-etcd-integration-1-cpu-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L311-L343) |
| pull-etcd-integration-2-cpu-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L345-L375) |
| pull-etcd-integration-2-cpu-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L377-L409) |
| pull-etcd-integration-4-cpu-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L411-L441) |
| pull-etcd-integration-4-cpu-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L443-L475) |
| pull-etcd-robustness-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L443-L475) |
| pull-etcd-robustness-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L555-L602) |
| pull-etcd-release-tests | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L604-L639) |
| pull-etcd-contrib-mixin | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L641-L669) |
| pull-etcd-fuzzing-v3rpc | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L671-L704) |
| pull-etcd-grpcproxy-integration-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L706-L735) |
| pull-etcd-grpcproxy-e2e-amd64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L737-L766) |
| pull-etcd-grpcproxy-integration-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L768-L798) |
| pull-etcd-grpcproxy-e2e-arm64 | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L768-L798) |
| pull-etcd-coverage-report | [link](https://github.com/kubernetes/test-infra/blob/563f6651a827f6f3e0b82db398cacb57f7ea74b3/config/jobs/etcd/etcd-presubmits.yaml#L832-L860) |

#### `release-3.5`

#### `release-3.4`
