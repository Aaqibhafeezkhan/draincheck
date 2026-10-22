# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Each entry should link to the pull request or issue that introduced it. Contributors who opt in are credited with their GitHub handle, e.g. `(#12, @username)`. Add new
entries under `[Unreleased]` in the same pull request as the change (see
[CONTRIBUTING.md](CONTRIBUTING.md)).

## [Unreleased]

### Added

- `--debug` flag. It writes one line to stderr for each action draincheck
  takes: the settings, every list call to the API server with its object count
  and duration, how the target nodes were resolved, each rule with its number
  of findings, the finding counts per severity and the exit code. The report on
  stdout is byte-for-byte the same with and without the flag, and the lines
  never contain tokens or kubeconfig contents (#21).

#### Analyzer rules

- `NO_REMAINING_NODES`: when the drain moves pods and no node is left that
  takes them, because every node is a target or the ones left are cordoned or
  not `Ready`, it is reported as `RISK`, one finding for the run. The drain
  completes, but the evicted pods stay `Pending` until a node can take them.
  Nothing is reported when no pod needs a new home. The JSON `evidence` has
  `pods_to_move`, `target_nodes`, `total_nodes` and `remaining_schedulable`
  (#43).
- `NODE_NOT_SCHEDULABLE`: when the drain moves pods, nodes outside the target
  set that are cordoned or not `Ready` are reported as `RISK`, one finding for
  all of them. The drain completes, but the evicted pods have fewer nodes to
  land on than the node count suggests. A target that is already cordoned is
  not reported, and nothing is reported when no pod needs a new home. The JSON
  `evidence` has `pods_to_move`, `remaining_nodes` and the state of each
  affected node (#42).
- `PDB_BATCH_EXCEEDS_BUDGET`: a PodDisruptionBudget that allows some
  disruptions, but fewer than the target nodes together would cause, is
  reported as `RISK`, one finding per budget. `kubectl drain` takes one node at
  a time and sees each fit; draincheck sums the pods of all target nodes that
  share the budget. The drain does not fail, it waits for replacements between
  evictions. A budget at 0 stays `BLOCKED` under `PDB_DISRUPTION_BLOCKED`. The
  JSON `evidence` has the budget fields of `PDB_DISRUPTION_BLOCKED` plus
  `pods_to_evict` (#12).
- `PDB_SYNC_FAILED`: a PodDisruptionBudget whose `DisruptionAllowed` condition
  has the reason `SyncFailed` while it selects pods on the target nodes is
  reported as `UNKNOWN`, one finding per budget. The disruption controller
  could not compute the budget, so the 0 allowed disruptions it records is not
  a real number. The JSON `evidence` has the budget fields of
  `PDB_DISRUPTION_BLOCKED` plus the condition `reason` and `message` (#100).
- `PDB_STALE_STATUS`: a PodDisruptionBudget whose `status.observedGeneration`
  differs from `metadata.generation` while it selects pods on the target nodes
  is reported as `UNKNOWN`, one finding per budget. Such a status describes an
  older spec, so its `disruptionsAllowed` cannot be trusted. The JSON
  `evidence` has the budget fields of `PDB_DISRUPTION_BLOCKED` (the numbers as
  last reported, `nodes` and `pods`) plus `observed_generation` and
  `generation` (#41).
- `SINGLE_REPLICA_WORKLOAD`: a Deployment or StatefulSet that desires one
  replica and has a pod on a target node is reported as `RISK`, one finding per
  workload. The pod is evicted before its replacement is Ready, so the workload
  is down in between, which `kubectl drain` does not warn about. Finished,
  DaemonSet and static pods are skipped. The JSON `evidence` has `owner_kind`,
  `owner_name`, `replicas` and `pods` (#40).

### Changed

- draincheck now also lists Deployments, StatefulSets, Jobs and CronJobs (all
  namespaces), so planned workload rules can see them. No rule uses them yet and
  the report does not change. The account that runs draincheck needs `list` on
  `deployments` and `statefulsets` (`apps`) and `jobs` and `cronjobs`
  (`batch`); a run without that permission stops with an error, it does not
  continue with an empty list (#38).
- draincheck now also lists PersistentVolumeClaims (all namespaces) and
  PersistentVolumes, so the planned volume rules can see them. No rule uses
  them yet and the report does not change. The account that runs draincheck
  needs `list` on `persistentvolumeclaims` and on `persistentvolumes`, which is
  cluster-scoped; a run without that permission stops with an error, it does
  not continue with an empty list (#39).
- `examples/rbac/` holds a read-only `ClusterRole` (`list` on every resource
  draincheck reads, no other verb) and a `ClusterRoleBinding` to adapt, so the
  account that runs draincheck can be set up without writing the rules by hand.

#### Analyzer rules

- `unhealthyPodEvictionPolicy` is honoured by the PodDisruptionBudget rules.
  A pod that is not Ready is evicted past its budget with `AlwaysAllow`, and
  with `IfHealthyBudget` (the default) while the budget is healthy. Such a pod
  is no longer counted as blocked: `PDB_DISRUPTION_BLOCKED` gives no finding
  for a budget at 0 whose only pods it lets through, and
  `PDB_BATCH_EXCEEDS_BUDGET` does not count them. With an unhealthy budget the
  pod is still refused and the finding says so. A budget with a policy value
  draincheck does not know is `UNKNOWN`. The JSON `evidence` gains
  `unhealthy_pod_eviction_policy` and `evictable_unhealthy_pods`, both omitted
  when empty (#5).
- `MISSING_RESOURCE_REQUESTS` now reports one finding per owning workload
  (Deployment, StatefulSet, Job or a pod with no controller) instead of one
  per pod, so 10 replicas without requests are one finding. The finding points
  at the workload and states how many of its pods on the target nodes are
  affected. The JSON `evidence` changes: `node` and `containers` move into a
  `pods` list (each pod with its `node` and `containers`), next to
  `owner_kind`, `owner_name` and `pods_on_targets`. No pod is dropped (#73).
- `POD_OWNERSHIP` now groups by cause instead of reporting every pod: one
  finding per DaemonSet, one per node for its static pods, and still one per
  pod with no controller. A DaemonSet on 28 nodes is one finding that says
  `--ignore-daemonsets` is needed, not 28 copies of the same advice. Findings
  point at the `DaemonSet` or the `Node` instead of a pod. The JSON `evidence`
  changes: `node` is replaced by `nodes` and a `pods` list (each pod with its
  `namespace`, `name` and `node`), next to `category` and, for DaemonSets,
  `owner_kind` and `owner_name`. No pod is dropped (#70).
- `PDB_DISRUPTION_BLOCKED` now reports one finding per PodDisruptionBudget
  instead of one per pod, so 10 replicas behind one budget are one finding.
  The finding points at the `PodDisruptionBudget` instead of a pod, and its
  description gives the numbers (healthy, desired) once, with how many pods on
  how many target nodes are affected. The JSON `evidence` gains `nodes` and a
  `pods` list (each pod with its `namespace`, `name` and `node`), next to the
  existing budget fields. No pod is dropped (#71).
- `PDB_DISRUPTION_BLOCKED` no longer reports a PodDisruptionBudget whose
  disruption controller failed to compute it (`PDB_SYNC_FAILED` reports it as
  `UNKNOWN`) (#100).
- `PDB_DISRUPTION_BLOCKED` no longer reports a PodDisruptionBudget whose status
  is stale as `BLOCKED`; `PDB_STALE_STATUS` reports it as `UNKNOWN` instead, so
  a drain is not failed on a number that may be out of date (#41).
- `PDB_DISRUPTION_BLOCKED` reports a PodDisruptionBudget that selects pods
  controlled by a custom resource (an operator) as `UNKNOWN` instead of
  `BLOCKED`, because whether such a budget stops the drain depends on operator
  state draincheck cannot see. The budgets of one operator in one namespace
  are one finding that points at the `Namespace`, so an operator with a budget
  per node gives one finding, not one per node. The JSON `evidence` of that
  finding is `managed_by` (`api_group`, `kind`), `namespace` and a `pdbs` list
  with the usual fields of each budget. Budgets over Deployments, StatefulSets,
  Jobs and the like stay `BLOCKED`. Findings of this kind make the run exit `1`
  only with `--fail-on any` (#72).

### Fixed

#### Analyzer rules

- `PDB_DISRUPTION_BLOCKED`, `PDB_BATCH_EXCEEDS_BUDGET` and
  `PDB_OVERLAPPING_SELECTORS` no longer count pods in phase `Pending` or pods
  that are already terminating. The eviction API deletes them without
  consulting a budget, so reporting them was a false `BLOCKED` (#5).
- `PDB_DISRUPTION_BLOCKED` and `PDB_OVERLAPPING_SELECTORS` no longer report
  DaemonSet pods and static pods. `kubectl drain` never evicts them, so no
  PodDisruptionBudget can refuse their eviction; a budget that selects only
  such pods was a false `BLOCKED`. They are still reported by `POD_OWNERSHIP`
  (#71).
- `NODE_NOT_SCHEDULABLE`, `NO_REMAINING_NODES` and `MISSING_RESOURCE_REQUESTS`
  no longer count pods that are already terminating. Their controller has
  already created the replacement, so the drain does not take them down; they
  inflated the number of pods to move and could add a workload to the
  missing-requests report. All rules now share one definition of which pods a
  drain acts on. `POD_OWNERSHIP` still reports a terminating DaemonSet, static
  or controller-less pod, because `kubectl drain` checks those too (#126).

## [0.0.1] - 2026-10-16

First pre-alpha release. It checks PodDisruptionBudgets, pod ownership and
missing resource requests before a multi-node drain, and reports in text or
JSON. Output and rules may still change before 0.1.0.

### Added

- CLI built on Cobra with `--version`, `--nodes`, `--kubeconfig`, `--context`,
  `--output` and `--fail-on` flags. Node names passed to `--nodes` are trimmed
  and empty names are rejected.
- Inventory collector (client-go) producing an immutable snapshot of nodes,
  pods, PodDisruptionBudgets and ReplicaSets, with pod owner resolution and
  validation of target nodes.
- Findings model: severity, resource reference and typed finding, with
  deterministic sorting by namespace, kind and name.
- Analyzer rule interface and runner with deterministic ordering (#2).
- JSON report formatter with `schema_version` `v1` and a golden-file test (#18).
- Human-readable text report formatter, used as the default output (#17).
- Pipeline wiring collector, analyzer and report, with exit codes: `1` when a
  finding reaches the `--fail-on` threshold, `2` when the run cannot complete
  (#20).
- Snapshot timestamp and mandatory disclaimer in every report: the text report
  gains a `Timestamp:` header and a closing disclaimer line, and the JSON
  report gains `generated_at` and `disclaimer` fields. The timestamp is when
  the cluster snapshot was taken, so the same snapshot gives identical output
  (#19).
- Compact text report: findings are grouped by rule and each remediation is
  printed once, long lists are collapsed. `--verbose` prints every finding in
  full (#74).
- Tag-triggered release workflow: signed multi-platform artifacts with SLSA
  provenance (#23).
- Project documentation: specification (`docs/SPEC.md`), `README.md`,
  `AGENTS.md`, `CONTRIBUTING.md`, pull request template, `SECURITY.md` (#24),
  `CODE_OF_CONDUCT.md` and CI documentation (`docs/CI.md`).
- CI: build, quality (vet, gofmt, mod tidy, race tests, golangci-lint) and PR
  validation workflows, with workflow linting via actionlint.
- Security automation: CodeQL, govulncheck, dependency review, OpenSSF
  Scorecard and Dependabot for Go modules and GitHub Actions.
- Apache-2.0 license, `Makefile` (build, test, vet, fmt) and golangci-lint
  configuration.

#### Analyzer rules

- `PDB_DISRUPTION_BLOCKED`: PDB with `disruptionsAllowed == 0` (#3).
- `PDB_OVERLAPPING_SELECTORS`: pod matched by more than one PDB (#4).
- `POD_OWNERSHIP`: DaemonSet, static and naked pods (#6).
- `MISSING_RESOURCE_REQUESTS`: pods without CPU or memory requests (#7).

### Changed

- Bump Go to 1.26 and refresh indirect dependencies.
- Bump `k8s.io/apimachinery` and `k8s.io/client-go` from 0.35.9 to 0.37.1.

[Unreleased]: https://github.com/Veinar/draincheck/compare/v0.0.1...develop
[0.0.1]: https://github.com/Veinar/draincheck/releases/tag/v0.0.1
