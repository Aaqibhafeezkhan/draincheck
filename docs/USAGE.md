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
  Pod/billing/payments-api-0: PodDisruptionBudget billing/payments-api-pdb allows 0 disruptions.
  Remediation: Add healthy replicas or relax the budget.

This report describes the observed cluster state at 2026-10-14T09:30:00Z and does not guarantee execution success. Cluster state may drift immediately after analysis.
```

- Findings of a rule that share a remediation print it once. A rule with several distinct remediations (for example `POD_OWNERSHIP` for DaemonSet versus static pods) prints each text once, with the resources it applies to: `Remediation (Pod/a/ds-1, Pod/a/ds-2): ...`.
- Long lists are collapsed after 5 entries and end in `... and M more`. A line at the end of the report says when anything was collapsed. `--verbose` prints every finding.
- `--verbose` prints one block per finding with its full description and remediation:

```text
[POD_OWNERSHIP] Pod/monitoring/agent-x1
  Pod monitoring/agent-x1 is managed by a DaemonSet.
  Remediation: Drain with --ignore-daemonsets.
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
      "resource": "Pod/billing/payments-api-0",
      "evidence": { "namespace": "billing", "name": "payments-api-pdb", "current_healthy": 3, "desired_healthy": 3, "disruptions_allowed": 0 },
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

Raised for a pod on a target node when a PodDisruptionBudget in the same namespace selects it and `status.disruptionsAllowed == 0`, so the eviction API will refuse the eviction.

- **Resource:** the pod (`Pod/<namespace>/<name>`), one finding per pod and PDB. The finding points at the pod because it is the thing the drain will be stuck on; the PDB is carried as evidence.
- **Evidence:** the PDB namespace and name, `current_healthy`, `desired_healthy` and `disruptions_allowed`.
- Only pods scheduled on a target node count. Pods in phase `Succeeded` or `Failed` are skipped, since eviction does not consult budgets for them.
- A PDB with an empty selector (`{}`) selects every pod in its namespace; one with no selector selects none.
- A PDB whose selector is invalid is skipped.
- Cluster-wide budgets summed across the batch (several nodes sharing one PDB) are *planned*.

### `PDB_OVERLAPPING_SELECTORS`

Severity: `BLOCKED`.

Raised for a pod on a target node when more than one PodDisruptionBudget in the same namespace selects it. The eviction API does not support a pod covered by several budgets and refuses the eviction, regardless of how many disruptions each budget allows.

- **Resource:** the pod (`Pod/<namespace>/<name>`), one finding per pod.
- **Evidence:** the `namespace` and the `pdbs` that select the pod, sorted by name.
- Only pods scheduled on a target node count. Pods in phase `Succeeded` or `Failed` are skipped, since eviction does not consult budgets for them.
- An empty selector (`{}`) selects every pod in its namespace, so it overlaps with any other budget there; one with no selector selects none.
- A PDB whose selector is invalid is skipped.
- A pod that is both overlapped and covered by a zero-disruption budget also gets a `PDB_DISRUPTION_BLOCKED` finding.

### `POD_OWNERSHIP`

Severity: `RISK`.

Raised for a pod on a target node whose ownership gives it special `kubectl drain` behaviour. One finding per pod; the `category` in the evidence says which:

| Category | Pod | What drain does |
| --- | --- | --- |
| `DaemonSet` | Controlled by a DaemonSet | Refused unless `--ignore-daemonsets`; never evicted. |
| `StaticPod` | API mirror of a kubelet static pod (annotation `kubernetes.io/config.mirror`) | Skipped; cannot be rescheduled, stops with the node. |
| `Unmanaged` | No controller | Refused unless `--force`; not recreated once deleted. |

- **Resource:** the pod (`Pod/<namespace>/<name>`).
- **Evidence:** `category`, `node`, and `owner_kind`/`owner_name` for DaemonSet pods.
- It is `RISK`, not `BLOCKED`, because whether the drain stops depends on flags draincheck cannot observe.
- Only pods scheduled on a target node count. Pods in phase `Succeeded` or `Failed` are skipped, since `kubectl drain` removes them without further checks.
- Pods managed by any other controller (Deployment, StatefulSet, Job, ...) are not reported. A pod whose ReplicaSet is missing from the snapshot is treated as managed.
- A pod with a mirror annotation is always `StaticPod`, even though it also has a `Node` owner reference.

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
