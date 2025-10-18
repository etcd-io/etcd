# Prow Jobs in etcd

## 1. Introduction to Prow
[Prow] is a Kubernetes based CI/CD system. Jobs can be triggered by various types of events and report their status to many different services. In addition to job execution, Prow provides GitHub automation in the form of policy enforcement, chat-ops via `/foo` style commands, and automatic PR merging.

When a user comments `/ok-to-test`or `/retest,` on a Pull Request, GitHub sends a webhook to Prow's Kubernetes cluster. The request travels through an ingress for TLS termination, gets routed to the hook service, and arrives at the hook application running in pods. Visit this [site][life-of-a-prow-job] to further understand the lifecycle of a Prow job.

This is where you can find all etcd Prow jobs status: https://prow.k8s.io/?repo=etcd-io%2Fetcd 

---

## 2. How Prow is used for etcd Testing
### where the jobs are located, i.e., test-infra repository, in the config/jobs/etcd directory

The CI of etcd is managed by kubernetes/test-infra, which leverages prow inside it.

Whenever a pull request is submitted, or a command is called, the CI of etcd, managed by [kubernetes/test-infra](https://github.com/kubernetes/test-infra) leverages Prow to run the tests. You can find all the supported commands [here](https://prow.k8s.io/command-help).

The history of the ran can be found [here](https://prow.k8s.io/job-history/gs/kubernetes-ci-logs/pr-logs/directory/pull-etcd-e2e-amd64?buildId= 
 ).


### Jobs Types
The jobs configuration for etcd is [here](https://github.com/kubernetes/test-infra/tree/master/config/jobs/etcd).

There are 3 different job types:
- Presubmits run against code in PRs
- Postsubmits run after merging code
- Periodics run on a periodic basis

Please see [ProwJob](https://docs.prow.k8s.io/docs/jobs/) docs for more info.



As an example, [here](https://github.com/kubernetes/test-infra/blob/master/config/jobs/etcd/etcd-presubmits.yaml) are the presubmits jobs of etcd. `pull-etcd-e2e-amd64` is one of the presubmits located [here](https://github.com/kubernetes/test-infra/blob/b21a1d3a72d5715ea7c9234cade21751847cfbe5/config/jobs/etcd/etcd-presubmits.yaml#L193). 


Refer to this [site](https://github.com/kubernetes/test-infra/tree/master/config/jobs#job-types) to learn more about job types.

### How to manually run a given job on Prow

These tests can be triggered when you leave a comment, like `/ok-to-test` or `/retest`, in PR [example](https://github.com/etcd-io/etcd/pull/20733#issuecomment-3341443205).
You can find all supported commands [here](https://prow.k8s.io/command-help).



## 3. Navigating Performance Dashboard (Grafana) (just navigating. I don't think we have an intention to create dashboards at this moment


### 3.1 Selecting our jobs, explaining the different kinds of workloads/jobs, i.e., robustness, integration, static check, etc. (at a high level).



## 4. Interpreting Metrics

---
[trigger-jobs]: https://docs.prow.k8s.io/docs/jobs/#triggering-jobs "Prow — Triggering jobs" 

[etcd-presubmits]: https://github.com/kubernetes/test-infra/blob/master/config/jobs/etcd/etcd-presubmits.yaml "etcd presubmits jobs"

[build-testing-updating-prow]: https://docs.prow.k8s.io/docs/build-test-update/#how-to-test-a-prowjob  "Prow developers and maintainers who want to build/test individual components or deploy changes to an existing Prow cluste"

[pr-interaction-sequence]: https://docs.prow.k8s.io/images/pr-interactions-sequence.svg "PR Interaction sequence diagram"

[life-of-a-prow-job]: https://docs.prow.k8s.io/docs/life-of-a-prow-job/ "Life of a Prow Job"

[prow]: https://github.com/kubernetes-sigs/prow "Prow"

[Prow status]: https://prow.k8s.io/ "Prow Status UI"