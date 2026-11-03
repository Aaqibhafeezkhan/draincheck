# draincheck

[![CI](https://github.com/Veinar/draincheck/actions/workflows/ci.yml/badge.svg?branch=develop)](https://github.com/Veinar/draincheck/actions/workflows/ci.yml)
[![Release](https://github.com/Veinar/draincheck/actions/workflows/release.yml/badge.svg)](https://github.com/Veinar/draincheck/actions/workflows/release.yml)
[![GitHub Release](https://img.shields.io/github/v/release/Veinar/draincheck?include_prereleases&sort=semver)](https://github.com/Veinar/draincheck/releases/latest)
![GitHub License](https://img.shields.io/github/license/Veinar/draincheck?color=%23dc9c1d)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/Veinar/draincheck/badge)](https://scorecard.dev/viewer/?uri=github.com/Veinar/draincheck)
[![Signed with cosign](https://img.shields.io/badge/signed-cosign%20keyless-blue)](docs/RELEASE.md#verify)
[![Read-only](https://img.shields.io/badge/cluster%20access-read--only-green)](docs/SPEC.md)

Read-only preflight analyzer for multi-node `kubectl drain`. `kubectl drain` tells you what is refused on one node, now. draincheck looks at the whole plan, several nodes at once, and tells you what would block it or put something at risk **before** you cordon anything.

> [!TIP]
> **First run?** Pass the same flags you will give `kubectl drain` (`--ignore-daemonsets`, `--delete-emptydir-data`, `--force`). Without them a cluster with DaemonSets exits `1`, because `kubectl drain` refuses those pods; re-running with the flags is the fix. Exit `0` does not mean the drain is safe: the default gates on `BLOCKED` only, so read the `RISK` findings too. See [docs/FIRST_RUN.md](docs/FIRST_RUN.md).

## Example

Output from a real cluster, with names anonymized. The text report is compact: findings are grouped by cause (one per PodDisruptionBudget, workload, DaemonSet or node, not one per pod), each remediation is printed once, a rule with more than 5 findings starts with a line such as `15 findings in 4 namespaces`, and a list is cut after 5 entries. `--verbose` prints every finding in full. The JSON report always carries every finding and every pod.

```text
$ draincheck --nodes worker-01 --ignore-daemonsets --delete-emptydir-data
draincheck - preflight maintenance analysis
Timestamp: 2026-11-03T08:20:21Z

=== FINDINGS SUMMARY ===
  BLOCKED: 0
  RISK:    47
  UNKNOWN: 1

=== RISK (47) ===

[MISSING_RESOURCE_REQUESTS] 15 findings in 4 namespaces, first 5 shown
  Deployment/cattle-fleet-system/fleet-controller: Missing resource requests in 1 of 1 pod of Deployment cattle-fleet-system/fleet-controller on the target nodes: fleet-controller (cpu, memory), fleet-cleanup (cpu, memory), fleet-agentmanagement (cpu, memory). The scheduler treats a missing request as needing nothing, so rescheduled pods may not fit where they land.
  Deployment/cattle-fleet-system/gitjob: Missing resource requests in 1 of 1 pod of Deployment cattle-fleet-system/gitjob on the target nodes: gitjob (cpu, memory). The scheduler treats a missing request as needing nothing, so rescheduled pods may not fit where they land.
  Deployment/cattle-fleet-system/helmops: Missing resource requests in 1 of 1 pod of Deployment cattle-fleet-system/helmops on the target nodes: helmops (cpu, memory). The scheduler treats a missing request as needing nothing, so rescheduled pods may not fit where they land.
  Deployment/kube-system/cilium-operator: Missing resource requests in 1 of 1 pod of Deployment kube-system/cilium-operator on the target nodes: cilium-operator (cpu, memory). The scheduler treats a missing request as needing nothing, so rescheduled pods may not fit where they land.
  Deployment/kube-system/hubble-relay: Missing resource requests in 1 of 1 pod of Deployment kube-system/hubble-relay on the target nodes: hubble-relay (cpu, memory). The scheduler treats a missing request as needing nothing, so rescheduled pods may not fit where they land.
  ... and 10 more

  => Remediation: Confirm with `kubectl get pod -n <namespace> <pod> -o jsonpath='{.spec.containers[*].resources.requests}'` for a pod named in the finding (empty output: no request is set; a LimitRange or webhook default shows here). Then set CPU and memory requests on every container, or a pod-level request, in the pod template of the workload so the scheduler and capacity checks can account for it. kubectl drain evicts these pods whatever they request; the risk is where they land.

[POD_LOCAL_STORAGE] 12 findings in 8 namespaces, first 5 shown
  Deployment/cattle-fleet-system/fleet-controller: Deployment cattle-fleet-system/fleet-controller has 1 pod with emptyDir volumes on 1 target node. With --delete-emptydir-data kubectl drain deletes the pods and the data in those volumes is lost; a controller recreates the pod with an empty volume.
  Deployment/cattle-fleet-system/gitjob: Deployment cattle-fleet-system/gitjob has 1 pod with emptyDir volumes on 1 target node. With --delete-emptydir-data kubectl drain deletes the pods and the data in those volumes is lost; a controller recreates the pod with an empty volume.
  Deployment/cattle-fleet-system/helmops: Deployment cattle-fleet-system/helmops has 1 pod with emptyDir volumes on 1 target node. With --delete-emptydir-data kubectl drain deletes the pods and the data in those volumes is lost; a controller recreates the pod with an empty volume.
  Deployment/shop-dev/shop-frontend: Deployment shop-dev/shop-frontend has 1 pod with emptyDir volumes on 1 target node. With --delete-emptydir-data kubectl drain deletes the pods and the data in those volumes is lost; a controller recreates the pod with an empty volume.
  Deployment/shop-test/shop-frontend: Deployment shop-test/shop-frontend has 1 pod with emptyDir volumes on 1 target node. With --delete-emptydir-data kubectl drain deletes the pods and the data in those volumes is lost; a controller recreates the pod with an empty volume.
  ... and 7 more

  => Remediation: Confirm with `kubectl get pod -n <namespace> <pod> -o jsonpath='{.spec.volumes}'` (an entry with emptyDir is local storage). Then, safest first: move data that must survive to a PersistentVolumeClaim before the drain; or accept the loss, which is what --delete-emptydir-data already says.

[POD_OWNERSHIP] 1 finding
  Node/worker-01: Node worker-01 runs 1 static pod (kube-system/kube-proxy-worker-01). Static pods cannot be evicted or rescheduled and stop when the node goes down.

  => Remediation: Confirm with `kubectl get pods -A --field-selector spec.nodeName=<node>` and `kubectl get pod -n <namespace> <pod> -o jsonpath='{.metadata.annotations}'` (the annotation kubernetes.io/config.mirror marks a static pod). Then plan for the static pods to be unavailable while the node is down; kubectl drain skips them.

[SINGLE_REPLICA_WORKLOAD] 19 findings in 10 namespaces, first 5 shown
  Deployment/cattle-fleet-system/fleet-controller: Deployment cattle-fleet-system/fleet-controller has 1 replica and it runs on node worker-01. The workload is unavailable from the eviction until the new pod is Ready; kubectl drain does not warn about it.
  Deployment/cattle-fleet-system/gitjob: Deployment cattle-fleet-system/gitjob has 1 replica and it runs on node worker-01. The workload is unavailable from the eviction until the new pod is Ready; kubectl drain does not warn about it.
  Deployment/cattle-fleet-system/helmops: Deployment cattle-fleet-system/helmops has 1 replica and it runs on node worker-01. The workload is unavailable from the eviction until the new pod is Ready; kubectl drain does not warn about it.
  Deployment/cattle-turtles-system/rancher-turtles-controller-manager: Deployment cattle-turtles-system/rancher-turtles-controller-manager has 1 replica and it runs on node worker-01. The workload is unavailable from the eviction until the new pod is Ready; kubectl drain does not warn about it.
  Deployment/shop-dev/shop-frontend: Deployment shop-dev/shop-frontend has 1 replica and it runs on node worker-01. The workload is unavailable from the eviction until the new pod is Ready; kubectl drain does not warn about it.
  ... and 14 more

  => Remediation: Confirm with `kubectl get <kind> -n <namespace> <name> -o jsonpath='{.spec.replicas}'` (1) and `kubectl get pdb -n <namespace>`. Then, safest first: run at least 2 replicas and wait until they are Ready, with a PodDisruptionBudget so a later drain keeps one up; or accept the downtime for this one. Replicas can also be raised by an autoscaler, which draincheck does not read.

=== UNKNOWN (1) ===

[PDB_DISRUPTION_BLOCKED] 1 finding
  Namespace/longhorn-system: Operator longhorn.io (InstanceManager) manages 2 PodDisruptionBudgets in namespace longhorn-system with 0 allowed disruptions, covering 2 pods on 1 target node. Whether the drain stalls depends on operator state draincheck cannot see, and the operator may remove such budgets when a node is cordoned.

  => Remediation: Budgets managed by operator longhorn.io: confirm the numbers with `kubectl get pdb -n <namespace>` (ALLOWED DISRUPTIONS) and see who manages a budget with `kubectl get pdb -n <namespace> <name> -o yaml` (labels, ownerReferences). Then check its drain documentation and the health of what it protects before draining. Do not relax minAvailable on them; the operator may recreate the budget. Longhorn can block a drain while the node holds the last healthy replica of a volume: check volume health and Longhorn's node drain policy setting before draining.

Some lists are collapsed. Use --verbose to print every finding.

This report describes the observed cluster state at 2026-11-03T08:20:21Z and does not guarantee execution success. Cluster state may drift immediately after analysis.
$ echo $?
0
```

The exit code is `0` here because no finding reaches the default `--fail-on blocked`. With `--fail-on risk` this run would exit `1`. See [docs/USAGE.md](docs/USAGE.md) for every flag, rule and exit code.

## What it is

- A read-only Go CLI built on `client-go`. It lists and reads cluster state; it never cordons, evicts or modifies anything.
- Findings are `BLOCKED`, `RISK` or `UNKNOWN`. There is deliberately no `SAFE`.
- Not a controller and not a scheduler simulator.
- What a server dry run (`kubectl drain --dry-run=server`) does not show, and draincheck reports (details in the [FAQ](#why-not-kubectl-drain---dry-run) and [docs/COMPARISON.md](docs/COMPARISON.md)):
  - several nodes against one PodDisruptionBudget (`PDB_BATCH_EXCEEDS_BUDGET`);
  - no node left to take the evicted pods, or only cordoned, not `Ready` or tainted ones (`NO_REMAINING_NODES`, `NODE_NOT_SCHEDULABLE`, `WORKLOAD_UNTOLERATED_TAINTS`);
  - a workload with one replica that goes down while its pod moves (`SINGLE_REPLICA_WORKLOAD`);
  - a workload with several replicas that all sit on the nodes you drain, so the batch takes it fully down (`ALL_REPLICAS_ON_TARGETS`).

## Install

Linux, macOS and Windows binaries (`amd64` and `arm64`) are on the [releases page](https://github.com/Veinar/draincheck/releases). Releases are signed with cosign (keyless) and ship SLSA provenance. Install and verification steps: [docs/RELEASE.md](docs/RELEASE.md). The krew plugin manifest (`kubectl drain-check`) is in the repository but not listed in the krew index yet (*planned*): [docs/KREW.md](docs/KREW.md).

## Usage

```sh
draincheck --nodes worker-02,worker-04 [--kubeconfig PATH] [--context NAME] [--output text|json] [--verbose] [--fail-on blocked|risk|any] [--ignore-daemonsets] [--delete-emptydir-data] [--force] [--pod-selector SELECTOR] [--disable-eviction] [--debug]
```

| Flag           | Description                                    | Default                |
| -------------- | ---------------------------------------------- | ---------------------- |
| `--nodes`      | Nodes to drain, comma-separated, or `'*'` for all (required) | none     |
| `--kubeconfig` | Path to kubeconfig                             | standard loading rules |
| `--context`    | Kubeconfig context                             | current context        |
| `--output`     | Output format: `text` or `json`                | `text`                 |
| `--verbose`    | Text output only: print every finding in full  | off                    |
| `--fail-on`    | Lowest finding level that exits `1`: `blocked`, `risk` or `any` | `blocked` |
| `--ignore-daemonsets`, `--force` | Same as in `kubectl drain`. Without them, DaemonSet pods and pods with no controller on a target node are `BLOCKED` (kubectl refuses them), so such a cluster exits `1`. Pass the flags you will pass to `kubectl drain` | off |
| `--delete-emptydir-data` | Same as in `kubectl drain`. Without it, a pod with an `emptyDir` volume on a target node is `BLOCKED` (kubectl refuses it); with it, `RISK` (the data is lost) | off |
| `--disable-eviction` | Same as in `kubectl drain`: pods are deleted, not evicted, so PodDisruptionBudgets are not consulted and the `PDB_*` rules report nothing | off |
| `--pod-selector` | Same as in `kubectl drain`: a label selector; pods that do not match are left out of the analysis | none |
| `--debug`      | Write one line to stderr for each action; the report on stdout does not change | off |

> [!TIP]
> Something looks wrong, or you want to report a problem? Run again with `--debug`. It writes a line to stderr for each action draincheck takes: the settings it uses, every API call with its object count and time, how the target nodes were resolved, each rule with its number of findings, and the exit code. The report on stdout stays the same, and the lines never contain tokens or kubeconfig contents. See [Debug output](docs/USAGE.md#debug-output).

> [!TIP]
> You usually do not need `--kubeconfig` or `--context`. draincheck loads your kubeconfig with the same rules as `kubectl`: the `KUBECONFIG` environment variable if set, otherwise `~/.kube/config`, using the current context. If `kubectl get nodes` works for the user running draincheck, `draincheck --nodes worker-02` works too. It does not call the `kubectl` binary; it reads the same kubeconfig and talks to the API server directly, so `kubectl` does not have to be installed.

## FAQ

### Why not `kubectl drain --dry-run`?

Use both. `kubectl drain --dry-run=server` asks the API server whether each eviction would be accepted now. It takes several node names in one run, but checks them one after another, each against the cluster as it is now. At that it is exact, and draincheck does not try to beat it: a budget that allows 0 disruptions shows up in both. draincheck is a different layer. `kubectl` answers "what happens if I drain this node now"; draincheck answers "what happens if I do this plan across several nodes, and what should I look at first".

What a server dry run cannot show, and draincheck reports:

- **Several nodes against one budget.** A dry-run eviction does not use up the budget, so every node passes, even when a real drain of the same nodes cannot finish (checked with kubectl v1.36.1: three nodes, one pod each, a budget allowing 1; the dry run exits 0, the real drain evicts the first pod, retries the others against the budget until `--timeout` runs out for each node, and exits 1 with those nodes left cordoned). `PDB_BATCH_EXCEEDS_BUDGET` adds up the pods of all target nodes behind one PodDisruptionBudget and reports a batch the budget allows only in steps.
- **Operators that react to a cordon.** A dry run does not really cordon, so it cannot show what an operator does when the node is cordoned. draincheck marks such budgets `UNKNOWN` instead of guessing. Checking what the operator does is *planned*.
- **What the drain leaves behind.** Static pods and DaemonSet pods, which kubectl never evicts and which stay until the node goes down.
- **Where the evicted pods can go.** A drain only evicts; it does not look at the other nodes. When every node is a target, or the nodes left are cordoned, not `Ready` or tainted where the pods do not tolerate it, the drain completes and the pods stay `Pending` (checked with kubectl v1.36.1; see [docs/COMPARISON.md](docs/COMPARISON.md)). `NO_REMAINING_NODES`, `NODE_NOT_SCHEDULABLE` and `WORKLOAD_UNTOLERATED_TAINTS` report it.
- **Downtime of a single replica.** kubectl evicts the only pod of a one-replica Deployment or StatefulSet without a warning (checked with kubectl v1.36.1). `SINGLE_REPLICA_WORKLOAD` names the workload.
- **What may not fit when rescheduled.** Evicted pods whose containers have no resource requests, because the scheduler treats a missing request as needing nothing.
- **All nodes in one run**, with exit codes for CI, and no retry loop.

draincheck reads a snapshot, so it cannot be more exact than the API server on a single eviction, and it does not predict whether evicted pods will fit elsewhere beyond cordons, readiness and taints: node selectors, affinity and capacity are not checked yet (*planned*).

The rule-by-rule comparison, including the cases where `kubectl drain` is ahead, is in [docs/COMPARISON.md](docs/COMPARISON.md).

## Build

```sh
go test ./...
go build -o draincheck ./cmd/draincheck
```

Requires Go 1.26.10+.

## Docs

[First run](docs/FIRST_RUN.md) · [Specification](docs/SPEC.md) · [Rules and noise budget](docs/RULES.md) · [Comparison with `kubectl drain --dry-run`](docs/COMPARISON.md) · [CI](docs/CI.md) · [Release and verification](docs/RELEASE.md)

## About this project

[![Claude](https://img.shields.io/badge/Claude-D97757?logo=claude&logoColor=fff)](#)

> [!IMPORTANT]
> **AI-assisted, not vibe-coded.** These days AI is part of how code gets written, and it is here too. A human stays in the loop for all of it: I read every change, run it, and commit it signed. The AI proposes, I decide.
>
> **Contributions the author cannot explain will be closed.** Use any tool you like, but you are accountable for every line you submit: understand it, run it, and be able to defend it in review. Vibe-coding is not welcome here.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[Apache-2.0](LICENSE)
