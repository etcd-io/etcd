# Prow Jobs in etcd

## 1. Introduction to Prow

[Prow](https://docs.prow.k8s.io/docs/) is a Kubernetes based CI/CD system. Jobs can be triggered by various types of events and report their status to many different services. In addition to job execution, Prow provides GitHub automation in the form of policy enforcement, chat-ops via `/foo` style commands, and automatic PR merging.

When a user comments `/ok-to-test`or `/retest,` on a Pull Request, GitHub sends a webhook to Prow's Kubernetes cluster. The request travels through an ingress for TLS termination, gets routed to the hook service, and arrives at the hook application running in pods. Visit this [site](https://docs.prow.k8s.io/docs/life-of-a-prow-job/) to further understand the lifecycle of a Prow job.
This is where you can find all etcd Prow jobs [status](https://prow.k8s.io/?repo=etcd-io%2Fetcd)

## 2. How Prow is used for etcd Testing

The CI of etcd is managed by kubernetes/test-infra, which leverages prow inside it.

Whenever a pull request is submitted, or a command is called, the CI of etcd, managed by [kubernetes/test-infra](https://github.com/kubernetes/test-infra) leverages Prow to run the tests. You can find all the supported [commands](https://prow.k8s.io/command-help).

The [history](https://prow.k8s.io/job-history/gs/kubernetes-ci-logs/pr-logs/directory/pull-etcd-e2e-amd64?buildId=)of the ran can be found.

### Jobs Types

The jobs [configuration](https://github.com/kubernetes/test-infra/tree/master/config/jobs/etcd) for etcd.

There are 3 different job types:

- Presubmits run against code in PRs
- Postsubmits run after merging code
- Periodics run on a periodic basis

Please see [ProwJob](https://docs.prow.k8s.io/docs/jobs/) docs for more info.

As an [example](https://github.com/kubernetes/test-infra/blob/master/config/jobs/etcd/etcd-presubmits.yaml), are the presubmits jobs of etcd. `pull-etcd-e2e-amd64` is one of the [presubmits](https://github.com/kubernetes/test-infra/blob/b21a1d3a72d5715ea7c9234cade21751847cfbe5/config/jobs/etcd/etcd-presubmits.yaml#L193).

Refer to [this](https://github.com/kubernetes/test-infra/tree/master/config/jobs#job-types) to learn more about job types.

### How to manually run a given job on Prow

These tests can be triggered when you leave a comment, like `/ok-to-test` or `/retest`, in PR [example](https://github.com/etcd-io/etcd/pull/20733#issuecomment-3341443205).
You can find all supported [commands](https://prow.k8s.io/command-help).

## 3. Navigating Performance Dashboard (Grafana)

One example of [dashboards](https://monitoring-gke.prow.k8s.io/d/96Q8oOOZk/builds?orgId=1&refresh=30s&var-org=etcd-io&var-repo=etcd&var-job=ci-etcd-robustness-main-amd64&var-build=All&from=now-7d&to=now) can be found.

This dashboard provides visibility into build resource usage (CPU, memory, number of running builds, etc.) for the Prow build cluster’s Kubernetes jobs. It is scoped via organization, repository, build identifier and time range filters.

### Panel: “Running / Pending Builds”

Shows the number of builds that are in Running vs Pending states over time.
Use it to track build backlog or concurrency — e.g., if the “Pending” line rises, builds may be waiting for resources.
If the “Running” line fluctuates a lot or remains at some steady value, you can infer how many builds typically run in parallel.

### Panel: “Memory Usage per Build”

Shows memory usage over time for each build ID (each build listed in the legend at the bottom).
The y‑axis shows memory use (e.g., in MiB / GiB).
Use this to spot builds with unusually high memory usage — a spike indicates one build consumed many resources.

### Panel: “CPU Usage per Build”

Similar to the memory panel but shows CPU usage per build over time. Spikes in CPU usage may indicate heavy compute jobs, inefficiencies, or need for resource tuning.

### Panel: "Resources"

- Memory panel

Green line (“used”): how much memory this build’s pod was using at each time point. Orange/Yellow line (“requested”): how much memory was requested (i.e., Kubernetes requests.memory) for that pod.
Red line (“limit”): how much memory was limited (i.e., Kubernetes limits.memory) for that pod.
Y‑axis: shows memory (GiB, MiB) over the build runtime.

X‑axis: time of day/date.
If the green “used” line is close to or hits the red “limit”, it means the build came close to its memory cap (risking OOM). If “used” is much lower than “requested”, you may be over‑allocating memory (waste).
If the “requested” line is much higher than “used”, it suggests the job’s request could be tuned downward.

- CPU panel

Similar structure: green = actual usage, orange/yellow = requested CPU, red = CPU limit (if set).
Y‑axis often in number of CPU cores or fraction thereof (e.g., 1.0 = one core).
A green line with spikes may show bursts of CPU usage (e.g., build or compile phases) while idle periods show low usage.
If CPU usage consistently saturates the limit, the job may be throttled or delayed. If usage is consistently far below request, tuning may reduce cost.

It is useful for a few reasons:

1. Tuning resources: By drilling into each build-run, you can determine realistic memory & CPU requests and limits for that job‑type. This helps avoid waste or avoid failed builds hitting resource limits.

2. Spotting anomalies: If one build suddenly used 8 GiB while normally this job uses 1 GiB, it may indicate a regression or mis‑configuration.

3. Capacity planning: Seeing typical and peak usage helps cluster operators plan node sizes, scheduling, concurrency of builds, etc.

4. Debugging performance issues: A build with unexpectedly high CPU or memory might be stuck, looping, or consuming resources inefficiently.

## 3.1 Prow job categories (robustness, integration, static checks)

- Robustness:
  - Description: Long-running, fault-injection and chaos-style end-to-end tests that validate etcd correctness and availability under failures (node crashes, network partitions, resource exhaustion, upgrades).
  - When to run: Periodics for continuous coverage; run for PRs that touch consensus, storage, recovery, or upgrade paths.
  - Example job patterns: pull-etcd-robustness, periodic-robustness

- Integration:
  - Description: Functional end-to-end and cross-component tests that exercise real client/server interactions, snapshots/restore, upgrades and compatibility across OS/arch.
  - When to run: Presubmits for PRs that change APIs, client behavior, or integration points; periodics for broad platform coverage.
  - Example job patterns: pull-etcd-e2e-amd64, pull-etcd-integration

- Static check:
  - Description: Fast, deterministic checks (build, unit tests, linters, go vet/staticcheck, formatting, license/header checks, generated-code verification) that catch style, correctness and packaging problems early.
  - When to run: Every PR as presubmits; quick feedback loop before running expensive tests.
  - Example job patterns: pull-etcd-verify, pull-etcd-lint, pull-etcd-unit

## 4. Interpreting Metrics

Some Prow components expose Prometheus metrics that can be used for monitoring and alerting. You can find metrics like the number of PRs in each Tide pool, a histogram of the number of PRs in each merge and various other metrics to this [site](https://github.com/kubernetes-sigs/prow/blob/main/site/content/en/docs/metrics/_index.md).
