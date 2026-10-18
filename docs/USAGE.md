# draincheck usage

> **Status: pre-alpha.** The collector, rules and the text and JSON reports are wired to the CLI. Sections marked *planned* describe intended behaviour.

## Synopsis

```sh
draincheck --nodes NODE[,NODE...] [--kubeconfig PATH] [--context NAME] [--output text|json] [--verbose] [--fail-on blocked|risk|any]
draincheck --version
```

## Flags

| Flag           | Default                | Description                                                  |
| -------------- | ---------------------- | ------------------------------------------------------------ |
| `--nodes`      | none (required)        | Nodes to drain, comma-separated. `'*'` selects every node.   |
| `--kubeconfig` | standard loading rules | Path to the kubeconfig file.                                 |
| `--context`    | current context        | Kubeconfig context to use.                                   |
| `--output`     | `text`                 | `text` (human-readable) or `json`. |
| `--verbose`    | off                    | Text output only: print every finding in full instead of the compact layout. JSON always carries every finding. |
| `--fail-on`    | `blocked`              | Lowest finding level that makes the run exit `1`: `blocked`, `risk` or `any`. |
| `--version`    |                        | Print the version and exit.                                  |

`'*'` cannot be combined with node names. Quote it so the shell does not expand it. Surrounding whitespace in names is trimmed; empty names are error.

## Examples

```sh
draincheck --nodes worker-02,worker-04
draincheck --nodes '*' --context staging --output json
draincheck --nodes worker-02 --output json --fail-on risk   # also fail CI on RISK
draincheck --nodes worker-02 --output json
```

## Findings

Each finding has exactly one severity. There is no `SAFE`.

| Severity  | Meaning                                                              |
| --------- | -------------------------------------------------------------------- |
| `BLOCKED` | A known constraint prevents the drain under the observed state.      |
| `RISK`    | A condition could cause disruption or scheduling friction.           |
| `UNKNOWN` | The outcome cannot be established reliably.                          |

Findings are sorted by namespace, kind and name.

## Text output

`--output text` (the default) writes a human-readable report to stdout. Findings are grouped by severity, `BLOCKED` first, then `RISK`, then `UNKNOWN`; sections without findings are omitted. Inside a section, findings are grouped by rule, rules in alphabetical order. A rule block has a header with the finding count, one line per finding and the remediation once. The output is deterministic: the same findings and snapshot time give byte-identical output.

```text
draincheck - preflight maintenance analysis
Timestamp: 2026-10-14T09:30:00Z

=== FINDINGS SUMMARY ===
  BLOCKED: 1
  RISK:    0
  UNKNOWN: 0

=== BLOCKED (1) ===

[PDB_DISRUPTION_BLOCKED] 1 finding
  PodDisruptionBudget/billing/payments-api-pdb: PodDisruptionBudget billing/payments-api-pdb allows 0 disruptions (3 healthy, 3 desired), so eviction of its 2 pods on 2 target nodes will be refused.
  Remediation: Add healthy replicas, or relax minAvailable/maxUnavailable on the PodDisruptionBudget, before draining.

This report describes the observed cluster state at 2026-10-14T09:30:00Z and does not guarantee execution success. Cluster state may drift immediately after analysis.
```

- Findings of a rule that share a remediation print it once. A rule with several distinct remediations (for example `POD_OWNERSHIP` for DaemonSet versus static pods) prints each text once, with the resources it applies to: `Remediation (DaemonSet/a/ds-1, DaemonSet/a/ds-2): ...`.
- Long lists are collapsed after 5 entries and end in `... and M more`. A line at the end of the report says when anything was collapsed. `--verbose` prints every finding.
- `--verbose` prints one block per finding with its full description and remediation:

```text
[POD_OWNERSHIP] DaemonSet/monitoring/agent
  DaemonSet monitoring/agent has 2 pods on 2 target nodes. kubectl drain refuses DaemonSet pods without --ignore-daemonsets and never evicts them.
  Remediation: Drain with --ignore-daemonsets; DaemonSet pods stay until the node itself goes down.
```

- `Timestamp` is when the cluster snapshot was taken (RFC 3339, UTC), not when the report was printed. The closing disclaimer is mandatory and is always the last line.
- With no findings the report says `No findings reported.` That is not a guarantee that the drain will succeed; there is no `SAFE` status.
- The cluster name and the target node list are *planned*.
- The layout is meant for people. Use `--output json` in scripts and CI.

## JSON output

`--output json` writes the document below to stdout. Errors go to stderr, so stdout is always either a complete document or empty.

```json
{
  "schema_version": "v1",
  "generated_at": "2026-10-14T09:30:00Z",
  "disclaimer": "This report describes the observed cluster state at 2026-10-14T09:30:00Z and does not guarantee execution success. Cluster state may drift immediately after analysis.",
  "findings": [
    {
      "rule_id": "PDB_DISRUPTION_BLOCKED",
      "severity": "BLOCKED",
      "resource": "PodDisruptionBudget/billing/payments-api-pdb",
      "evidence": {
        "namespace": "billing",
        "name": "payments-api-pdb",
        "current_healthy": 3,
        "desired_healthy": 3,
        "disruptions_allowed": 0,
        "nodes": ["worker-02", "worker-03"],
        "pods": [
          { "namespace": "billing", "name": "payments-api-0", "node": "worker-02" },
          { "namespace": "billing", "name": "payments-api-1", "node": "worker-03" }
        ]
      },
      "description": "...",
      "remediation": "..."
    }
  ]
}
```

- `schema_version` is `"v1"`. Any change to the document shape is a deliberate, reviewed change, pinned by the golden file `testdata/report/json-v1.golden.json`.
- `findings` is always an array, empty when there are none. It is sorted by namespace, kind, name, rule ID and severity, so the same findings and snapshot time give byte-identical output.
- `resource` is `Kind/namespace/name`, or `Kind/name` for cluster-scoped objects.
- `evidence` is a typed object whose shape depends on `rule_id` (see *Rules*); it is `null` when a rule has none.
- `generated_at` is the time the cluster snapshot was taken (RFC 3339, UTC), not the time the document was written, so the same snapshot always gives the same bytes.
- `disclaimer` is the mandatory statement that the report does not guarantee execution success. It names the same timestamp.

## Rules

Rules are implemented in `internal/analyzer` and all of them run on every invocation. How each rule keeps its noise down (false-positive cases, downgrades, grouping) is recorded in [RULES.md](RULES.md).

### `PDB_DISRUPTION_BLOCKED`

Severity: `BLOCKED`.

Raised for a PodDisruptionBudget with `status.disruptionsAllowed == 0` that selects pods on the target nodes, so the eviction API will refuse to evict them. Findings are grouped by budget: ten replicas behind one PDB are one finding, not ten.

- **Resource:** the PDB (`PodDisruptionBudget/<namespace>/<name>`), one finding per PDB.
- **Description:** states the numbers once (healthy, desired, 0 allowed) and how many pods on how many target nodes are affected.
- **Evidence:** the PDB `namespace` and `name`, `current_healthy`, `desired_healthy` and `disruptions_allowed`, then `nodes` (the target nodes the selected pods run on, sorted) and `pods`, each with its `namespace`, `name` and `node`, sorted by namespace and name. Every selected pod is listed, so grouping hides nothing. A pod selected by two blocked PDBs is listed under both.
- Only pods scheduled on a target node count. Pods in phase `Succeeded` or `Failed`, DaemonSet-managed pods and static (mirror) pods are skipped, since `kubectl drain` does not evict them and the eviction API is never asked about them. They stay visible through `POD_OWNERSHIP`. A PDB that selects only such pods gives no finding.
- A PDB with an empty selector (`{}`) selects every pod in its namespace; one with no selector selects none.
- A PDB whose selector is invalid is skipped.
- A PDB whose status is stale is not reported here: `PDB_STALE_STATUS` reports it as `UNKNOWN`, because its numbers cannot be trusted.
- Cluster-wide budgets summed across the batch (several nodes sharing one PDB) are *planned*.

### `PDB_OVERLAPPING_SELECTORS`

Severity: `BLOCKED`.

Raised for a pod on a target node when more than one PodDisruptionBudget in the same namespace selects it. The eviction API does not support a pod covered by several budgets and refuses the eviction, regardless of how many disruptions each budget allows.

- **Resource:** the pod (`Pod/<namespace>/<name>`), one finding per pod.
- **Evidence:** the `namespace` and the `pdbs` that select the pod, sorted by name.
- Only pods scheduled on a target node count. Pods in phase `Succeeded` or `Failed` are skipped, since eviction does not consult budgets for them. So are DaemonSet-managed pods and static (mirror) pods, which `kubectl drain` never evicts. They stay visible through `POD_OWNERSHIP`.
- An empty selector (`{}`) selects every pod in its namespace, so it overlaps with any other budget there; one with no selector selects none.
- A PDB whose selector is invalid is skipped.
- A pod that is both overlapped and covered by a zero-disruption budget is also listed in the `PDB_DISRUPTION_BLOCKED` finding of that budget.

### `PDB_STALE_STATUS`

Severity: `UNKNOWN`.

Raised for a PodDisruptionBudget whose `status.observedGeneration` differs from `metadata.generation` while it selects pods on the target nodes. The status of such a budget describes an older spec, so its numbers, `disruptionsAllowed` included, cannot be trusted, and draincheck cannot say whether the drain will be refused. The finding replaces the `PDB_DISRUPTION_BLOCKED` one for the same budget. It is `UNKNOWN`, so it makes the run exit `1` only with `--fail-on any`.

- **Resource:** the PDB (`PodDisruptionBudget/<namespace>/<name>`), one finding per PDB.
- **Description:** states `status.observedGeneration` and `metadata.generation`, and how many pods on how many target nodes are affected.
- **Evidence:** the same fields as `PDB_DISRUPTION_BLOCKED` (the numbers as last reported, `nodes` and `pods`) plus `observed_generation` and `generation`.
- A stale PDB is reported whatever its status allows, since an out-of-date number that allows a disruption cannot be trusted either.
- The pods counted are the same as for `PDB_DISRUPTION_BLOCKED`: only pods on a target node, without finished, DaemonSet-managed and static pods. A stale PDB that selects only such pods gives no finding.
- A PDB whose status is only briefly behind (right after its spec was edited) clears up once the disruption controller catches up; run draincheck again. One that stays stale points to an unhealthy kube-controller-manager.
- Reporting a PDB whose selector matches no pod at all (probably a typo) is *planned*.

### `POD_OWNERSHIP`

Severity: `RISK`.

Raised for pods on a target node whose ownership gives them special `kubectl drain` behaviour. Findings are grouped by cause, so a DaemonSet that runs on many nodes is one finding, not one per pod. The `category` in the evidence says which kind it is:

| Category | One finding per | What drain does |
| --- | --- | --- |
| `DaemonSet` | DaemonSet, covering all its pods on the target nodes | Refused unless `--ignore-daemonsets`; never evicted. |
| `StaticPod` | Node, covering the API mirrors of kubelet static pods on it (annotation `kubernetes.io/config.mirror`) | Skipped; cannot be rescheduled, stops with the node. |
| `Unmanaged` | Pod with no controller | Refused unless `--force`; not recreated once deleted. |

- **Resource:** `DaemonSet/<namespace>/<name>` for DaemonSet pods, `Node/<name>` for static pods and `Pod/<namespace>/<name>` for a pod with no controller.
- **Evidence:** `category`, `nodes` (the nodes the covered pods run on, sorted) and `pods`, each with its `namespace`, `name` and `node`, sorted by namespace and name. A DaemonSet finding also has `owner_kind` and `owner_name`. Every covered pod is listed, so grouping hides nothing.
- Static pods get one finding per node rather than one for the whole drain, because the node is what you choose with `--nodes`. The description names the static pods; the point is to plan around the node outage, since kubectl skips them.
- It is `RISK`, not `BLOCKED`, because whether the drain stops depends on flags draincheck cannot observe.
- Only pods scheduled on a target node count. Pods in phase `Succeeded` or `Failed` are skipped, since `kubectl drain` removes them without further checks.
- Pods managed by any other controller (Deployment, StatefulSet, Job, ...) are not reported. A pod whose ReplicaSet is missing from the snapshot is treated as managed.
- A pod with a mirror annotation is always `StaticPod`, even though it also has a `Node` owner reference.
- A pod controlled by a DaemonSet that no longer exists is still reported as a DaemonSet pod, since draincheck does not list DaemonSets. `kubectl drain` handles such an orphan differently: it fails with a not-found error, and only `--force` deletes the pod. Handling this case is *planned*.

### `MISSING_RESOURCE_REQUESTS`

Severity: `RISK`.

Raised for a workload with a pod on a target node that has at least one regular container that declares no CPU request or no memory request. The scheduler treats a missing request as zero, so the pod can be placed on a node with no real room for it, and capacity checks cannot account for it.

- **Resource:** the owning workload (for example `Deployment/<namespace>/<name>`, `StatefulSet/...`, `Job/...`), one finding per workload however many replicas are affected. A pod with no controller is its own workload (`Pod/<namespace>/<name>`). A pod owned by a ReplicaSet resolves to the Deployment that owns the ReplicaSet.
- **Description:** states how many of the workload's pods on the target nodes are affected, for example `2 of 3 pods of Deployment shop/web`.
- **Evidence:** `owner_kind` and `owner_name` (omitted for a pod with no controller), `pods_on_targets` (the workload's pods on the target nodes that `kubectl drain` would evict, with or without requests) and `pods`. Each entry of `pods` has the pod `name`, its `node` and its `containers`: each container with a missing request, in spec order, with `missing_cpu` and `missing_memory`. Pods are sorted by name. The pod namespace is the namespace of the resource.
- Only pods scheduled on a target node count, in `pods` and in `pods_on_targets`. Pods in phase `Succeeded` or `Failed`, DaemonSet-managed pods and static (mirror) pods are skipped, since `kubectl drain` does not evict them and they are never rescheduled. They stay visible through `POD_OWNERSHIP`.
- Init containers are not inspected.
- A CPU or memory request set at pod level (`spec.resources.requests`) covers every container for that resource.
- Requests are read from the stored pod spec, so values filled in by a LimitRange or an admission webhook count as set.

## Cluster access

Read-only. Needs `list` on:

- `nodes`
- `pods` (all namespaces)
- `replicasets` (`apps`, all namespaces)
- `poddisruptionbudgets` (`policy`, all namespaces)

draincheck never cordons, evicts or modifies anything.

## Exit codes

| Code | Meaning                                                                 |
| ---- | ----------------------------------------------------------------------- |
| `0`  | Analysis completed; no finding at or above the `--fail-on` level.       |
| `1`  | Analysis completed; at least one finding at or above `--fail-on`.       |
| `2`  | draincheck could not complete: bad flags, unreadable kubeconfig, cluster unreachable, unknown node. |

`--fail-on` picks what counts:

| Value     | Exits `1` on                       |
| --------- | ---------------------------------- |
| `blocked` | `BLOCKED` (default)                |
| `risk`    | `BLOCKED` or `RISK`                |
| `any`     | any finding, including `UNKNOWN`   |

The report is written before the exit code is decided, so a run that exits `1` still prints every finding.

## See also

- [Specification](SPEC.md)
