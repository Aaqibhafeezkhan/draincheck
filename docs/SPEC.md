# draincheck: Project Specification

**Status:** Draft / Proposed v0.1
**License:** 100% free and open source (Apache-2.0). No paid tiers, no commercial upsells, no enterprise editions.

## 1. The core problem: the Kubernetes maintenance blind spot

Every Kubernetes operator eventually needs to patch, upgrade or reboot a set of worker nodes (for example `worker-02` and `worker-04`). They run `kubectl drain` on each, and halfway through the operation stalls or causes unexpected downtime.

`kubectl drain` is an executing command, not a planning tool. It evicts pods one by one and discovers blockers only when it hits them:

- A PodDisruptionBudget (PDB) is misconfigured or allows 0 disruptions, which halts the drain.
- A legacy deployment has no resource requests, so rescheduling is rejected or unpredictable.
- Hard pod anti-affinity rules leave evicted pods with nowhere to land.
- Several nodes are drained at once, starving the remaining nodes of CPU and memory.

### The gap

There is no standard, read-only preflight tool that evaluates a multi-node maintenance plan across the whole cluster before a single node is tainted, cordoned or drained. Operators write brittle bash scripts, or find out at 2:00 AM in production.

## 2. What draincheck is, and is not

### It is

- A **read-only CLI tool written in Go** that inspects cluster state through the standard Kubernetes client libraries (`client-go`).
- A **preflight risk and constraint analyzer**: it reads cluster state, analyzes a target list of nodes, and outputs a structured report of known blockers, configuration risks and blind spots.
- Designed for **CI/CD and runbooks**: clean JSON plus a human-readable summary, so it can run in a pull request or a pre-maintenance checklist.

### It is not

- **An active controller.** It never cordons nodes, evicts pods, modifies PDBs or mutates cluster state. It is strictly non-destructive.
- **A full scheduler simulator.** Replicating `kube-scheduler` logic creates false confidence. draincheck validates hard static constraints and labels complex or unknown cases as `UNKNOWN`.
- **A commercial product.** It is a community utility built by operators, for operators.

## 3. Design philosophy: no false confidence

draincheck deliberately has no `SAFE` status. In a distributed system you cannot guarantee success in advance. Every finding is in exactly one of three states:

| State     | Definition                                                                                                          | Example                                                                                  |
| --------- | ------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `BLOCKED` | A known constraint definitively prevents or rejects the planned operation under the observed state.                 | A PDB requires 3 healthy replicas, only 3 exist, so 0 disruptions are allowed.           |
| `RISK`    | A configuration or workload condition could cause disruption, downtime or scheduling friction.                      | A deployment lacks resource requests, or relies on host-port bindings.                   |
| `UNKNOWN` | The tool cannot establish the outcome reliably due to unsupported constraints, custom webhooks or dynamic races.    | A custom scheduler plugin or complex cloud storage attachment topology is in use.        |

### 3.1 What `BLOCKED` and `RISK` mean for a drain that retries

`kubectl drain` does not give up on a refused eviction: it retries until `--timeout`, which is infinite by default. The two states are therefore defined by whether waiting can fix the problem:

- `BLOCKED`: the drain will not complete from the observed state without outside action (a change to the cluster, the workload or the budget). Waiting does not fix it.
- `RISK`: the drain proceeds, possibly in steps or after waiting, but something is lost, degraded or delayed.
- `UNKNOWN`: draincheck cannot establish which of the two applies.

A PodDisruptionBudget at 0 is the reference case for `BLOCKED`. The disruption controller sets `disruptionsAllowed` to `currentHealthy - desiredHealthy`. At 0 the eviction is refused and the pod stays, so no replacement is created and `currentHealthy` does not rise because of the drain; only something outside the drain can lift it. A budget that allows at least one disruption is different: the first eviction frees a replacement, the budget recovers when that replacement is Ready, and the drain proceeds in steps.

| Situation | State | Why |
| --------- | ----- | --- |
| A budget selects pods on the target nodes and allows 0 disruptions | `BLOCKED` | The eviction is refused and retrying does not change the count. |
| A budget allows 1 disruption, the batch evicts 3 pods of it | `RISK` (`PDB_BATCH_EXCEEDS_BUDGET`) | The drain waits for each replacement to be Ready; it proceeds. |
| Every node a workload may run on is drained | `RISK` (*planned*) | The drain itself completes; the evicted pods stay Pending and the workload is down. |
| The numbers needed to tell (a stale budget status, an operator-managed budget) are not reliable | `UNKNOWN` | Nothing can be said about the outcome. |

The report is a snapshot, as the disclaimer below says. A state that clears by itself, such as a rollout finishing or a pod becoming Ready, is not seen.

Every report carries this mandatory disclaimer:

> This report describes the observed cluster state at timestamp T and does not guarantee execution success. Cluster state may drift immediately after analysis.

## 4. v0.1 scope

v0.1 focuses on five checks:

1. **PDB constraints**: disruption budgets that may prevent or delay eviction; correct handling of overlapping selectors (a pod matching multiple PDBs) and unhealthy pod states.
2. **Pod ownership**: DaemonSet-managed pods, static/mirror pods and unmanaged naked pods, which have special drain or eviction behavior.
3. **Placement constraints**: hard node selectors, required node affinity, taints/tolerations and required pod anti-affinity.
4. **Capacity uncertainty**: whether eligible destination nodes have enough allocatable CPU and memory for affected pods, explicitly disclaiming full scheduler priority ranking.
5. **Multi-node grouping**: the proposed set of target nodes assessed together, catching shared disruption budget conflicts across the whole maintenance batch.

## 5. Architecture and code layout

```text
draincheck/
├── cmd/draincheck/          # CLI entrypoint (Cobra flags and execution)
├── internal/
│   ├── inventory/           # Read-only ingestion of nodes, pods, PDBs and storage
│   ├── analyzer/            # Pure, stateless rule evaluation logic
│   ├── findings/            # Strongly typed finding definitions (BLOCKED, RISK, UNKNOWN)
│   └── report/              # Human-readable console output and JSON formatter
├── testdata/                # Mock Kubernetes objects for unit tests
└── tests/                   # Integration tests (fake client, envtest, kind)
```

### Key architectural rules

- **Typed evidence.** Rules return concrete, versioned Go structs, not `map[string]interface{}`, so the JSON schema stays stable for CI pipelines.
- **Immutable snapshots.** The inventory collector takes a read-only snapshot of cluster state; rules evaluate that snapshot deterministically.
- **Sorted findings.** All findings are sorted consistently by namespace, resource kind and name before printing or serialization.
- **Accountable actions.** Every action draincheck takes is accounted for by a `DEBUG` line when `--debug` is set: each cluster read, each rule run and each decision that shapes the result. The lines go to stderr through an injected `*slog.Logger`, never to stdout, and never carry tokens or kubeconfig contents. See [USAGE.md](USAGE.md#debug-output).
- **Exit codes are the CI/CD contract.** `0`: the analysis completed and no finding reaches `--fail-on`. `1`: the analysis completed and at least one finding reaches `--fail-on`. `2`: draincheck could not complete (bad flags, unreadable kubeconfig, unreachable cluster, unknown node). The report is written before the code is decided. `--debug` never changes the code. One limit applies: when stdout or stderr is a pipe whose reader has already exited, Go ends the process with `SIGPIPE` and the shell reports `141` in place of `0`, `1` or `2`. This applies to stdout with or without `--debug`, and to stderr only with `--debug`, because without it stderr is written only on failure. A gate must not pipe either stream into a command that quits early, and should treat `141` as a failed run. See [USAGE.md](USAGE.md#exit-codes).

## 6. Example output

### Human-readable console output

```text
$ draincheck --nodes "worker-02,worker-04"

[draincheck v0.1.0] - Preflight Maintenance Analysis
Cluster: production-lon-01 | Timestamp: 2026-10-10T21:00:00Z
Target Nodes: [worker-02, worker-04]

=== FINDINGS SUMMARY ===
  BLOCKED: 1
  RISK:    2
  UNKNOWN: 1

...
```