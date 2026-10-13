# draincheck usage

> **Status: pre-alpha.** Flags are parsed and validated; analysis and reports are not implemented yet. Sections marked *planned* describe intended behaviour.

## Synopsis

```sh
draincheck --nodes NODE[,NODE...] [--kubeconfig PATH] [--context NAME] [--output text|json]
draincheck --version
```

## Flags

| Flag           | Default                | Description                                                  |
| -------------- | ---------------------- | ------------------------------------------------------------ |
| `--nodes`      | none (required)        | Nodes to drain, comma-separated. `'*'` selects every node.   |
| `--kubeconfig` | standard loading rules | Path to the kubeconfig file.                                 |
| `--context`    | current context        | Kubeconfig context to use.                                   |
| `--output`     | `text`                 | `text` or `json`.                                            |
| `--version`    |                        | Print the version and exit.                                  |

`'*'` cannot be combined with node names. Quote it so the shell does not expand it. Surrounding whitespace in names is trimmed; empty names are error.

## Examples

```sh
draincheck --nodes worker-02,worker-04
draincheck --nodes '*' --context staging
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

## Rules

Rules are implemented in `internal/analyzer`; they are not yet wired into the CLI (*planned*).

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

## Cluster access

Read-only. Needs `list` on:

- `nodes`
- `pods` (all namespaces)
- `replicasets` (`apps`, all namespaces)
- `poddisruptionbudgets` (`policy`, all namespaces)

draincheck never cordons, evicts or modifies anything.

## Exit codes (*planned*)

| Code | Meaning                  |
| ---- | ------------------------ |
| `0`  | No blocking findings     |
| `1`  | At least one `BLOCKED`   |
| `2`  | Error                    |

Whether `RISK` also fails the run is undecided.

## See also

- [Specification](SPEC.md)
