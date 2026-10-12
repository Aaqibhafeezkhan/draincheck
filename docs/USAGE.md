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

## Cluster access

Read-only. Needs `list` on:

- `nodes`
- `pods` (all namespaces)
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
