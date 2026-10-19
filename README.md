# draincheck

![GitHub Created At](https://img.shields.io/github/created-at/Veinar/draincheck?color=%234ecca3)
![GitHub last commit](https://img.shields.io/github/last-commit/Veinar/draincheck)
[![Go](https://img.shields.io/badge/Go-%2300ADD8.svg?&logo=go&logoColor=white)](#)
![GitHub top language](https://img.shields.io/github/languages/top/Veinar/draincheck)
![GitHub License](https://img.shields.io/github/license/Veinar/draincheck?color=%23dc9c1d)
[![GitHub Release](https://img.shields.io/github/v/release/Veinar/draincheck?include_prereleases&sort=semver)](https://github.com/Veinar/draincheck/releases/latest)
![GitHub Downloads](https://img.shields.io/github/downloads/Veinar/draincheck/total)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/Veinar/draincheck?color=%233c5c68)
![Platforms](https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-lightgrey)
![Contrib welcome](https://img.shields.io/badge/contributions-welcome-blue)

[![CI](https://github.com/Veinar/draincheck/actions/workflows/ci.yml/badge.svg?branch=develop)](https://github.com/Veinar/draincheck/actions/workflows/ci.yml)
[![CodeQL](https://github.com/Veinar/draincheck/actions/workflows/codeql.yml/badge.svg?branch=develop)](https://github.com/Veinar/draincheck/actions/workflows/codeql.yml)
[![govulncheck](https://github.com/Veinar/draincheck/actions/workflows/govulncheck.yml/badge.svg?branch=develop)](https://github.com/Veinar/draincheck/actions/workflows/govulncheck.yml)
[![Release](https://github.com/Veinar/draincheck/actions/workflows/release.yml/badge.svg)](https://github.com/Veinar/draincheck/actions/workflows/release.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/Veinar/draincheck/badge)](https://scorecard.dev/viewer/?uri=github.com/Veinar/draincheck)

[![Signed with cosign](https://img.shields.io/badge/signed-cosign%20keyless-blue)](docs/RELEASE.md#verify)
[![Read-only](https://img.shields.io/badge/cluster%20access-read--only-green)](docs/SPEC.md)

[![Claude](https://img.shields.io/badge/Claude-D97757?logo=claude&logoColor=fff)](#)

> [!NOTE]
> **AI-assisted, not vibe-coded.** AI helps with code; every change is reviewed and committed by a human.

Read-only preflight analyzer for multi-node `kubectl drain`. Tells you what would block or put at risk a planned maintenance **before** you cordon anything.

> [!WARNING]
> `--verbose` prints every finding in full, one block per pod with its own description and remediation. On a real cluster that can be hundreds of lines (one 5-node run produced 95 findings). The default text report is compact: findings are grouped by rule, each remediation is printed once and long lists are collapsed after 5 entries. Findings are still reported per pod; grouping per workload is *planned*.

## What it is

- A read-only Go CLI built on `client-go`. It never cordons, evicts or modifies anything.
- Findings are `BLOCKED`, `RISK` or `UNKNOWN`. There is deliberately no `SAFE`.

## What it is not

- Not a controller.
- Not a scheduler simulator.

## Usage

```sh
draincheck --nodes worker-02,worker-04 [--kubeconfig PATH] [--context NAME] [--output text|json] [--verbose] [--fail-on blocked|risk|any] [--debug]
```

| Flag           | Description                                    | Default                |
| -------------- | ---------------------------------------------- | ---------------------- |
| `--nodes`      | Nodes to drain, comma-separated, or `'*'` for all (required) | none     |
| `--kubeconfig` | Path to kubeconfig                             | standard loading rules |
| `--context`    | Kubeconfig context                             | current context        |
| `--output`     | Output format: `text` or `json`                | `text`                 |
| `--verbose`    | Text output only: print every finding in full  | off                    |
| `--fail-on`    | Lowest finding level that exits `1`: `blocked`, `risk` or `any` | `blocked` |
| `--debug`      | Write one line to stderr for each action; the report on stdout does not change | off |

> [!TIP]
> Something looks wrong, or you want to report a problem? Run again with `--debug`. It writes a line to stderr for each action draincheck takes: the settings it uses, every API call with its object count and time, how the target nodes were resolved, each rule with its number of findings, and the exit code. The report on stdout stays the same, and the lines never contain tokens or kubeconfig contents. See [Debug output](docs/USAGE.md#debug-output).

> [!TIP]
> You usually do not need `--kubeconfig` or `--context`. draincheck loads your kubeconfig with the same rules as `kubectl`: the `KUBECONFIG` environment variable if set, otherwise `~/.kube/config`, using the current context. If `kubectl get nodes` works for the user running draincheck, `draincheck --nodes worker-02` works too. It does not call the `kubectl` binary; it reads the same kubeconfig and talks to the API server directly, so `kubectl` does not have to be installed.

## Example

> [!NOTE]
> Output from a real cluster, with names anonymized and the list shortened (a real run prints at most 5 findings per rule and then `... and M more`).

```text
$ draincheck --nodes worker-02
draincheck - preflight maintenance analysis
Timestamp: 2026-10-15T20:06:57Z

=== FINDINGS SUMMARY ===
  BLOCKED: 1
  RISK:    7
  UNKNOWN: 0

=== BLOCKED (1) ===

[PDB_DISRUPTION_BLOCKED] 1 finding
  Pod/longhorn-system/instance-manager-9c1e7a4b: PodDisruptionBudget longhorn-system/instance-manager-9c1e7a4b allows 0 disruptions (1 healthy, 1 desired), so eviction of pod instance-manager-9c1e7a4b from node worker-02 will be refused.
  Remediation: Add healthy replicas, or relax minAvailable/maxUnavailable on the PodDisruptionBudget, before draining.

=== RISK (7) ===

[MISSING_RESOURCE_REQUESTS] 4 findings
  Pod/cert-manager/cert-manager-cainjector-5d8f7c9b6-x2k4p: Pod cert-manager/cert-manager-cainjector-5d8f7c9b6-x2k4p on node worker-02 has containers without resource requests: cert-manager-cainjector (cpu); the scheduler treats them as needing nothing, so the pod may not fit where it is rescheduled.
  Pod/cert-manager/cert-manager-webhook-7b6d9f5c4-q8m2z: Pod cert-manager/cert-manager-webhook-7b6d9f5c4-q8m2z on node worker-02 has containers without resource requests: cert-manager-webhook (cpu); the scheduler treats them as needing nothing, so the pod may not fit where it is rescheduled.
  Pod/monitoring/grafana-6c9d8b7f5-4tn7w: Pod monitoring/grafana-6c9d8b7f5-4tn7w on node worker-02 has containers without resource requests: grafana (cpu, memory); the scheduler treats them as needing nothing, so the pod may not fit where it is rescheduled.
  Pod/shop/web-7f565f8df8-glh5k: Pod shop/web-7f565f8df8-glh5k on node worker-02 has containers without resource requests: web (cpu, memory), worker (cpu, memory); the scheduler treats them as needing nothing, so the pod may not fit where it is rescheduled.
  Remediation: Set CPU and memory requests on every container (or a pod-level request) so rescheduling and capacity checks can account for the pod.

[POD_OWNERSHIP] 3 findings
  Pod/kube-system/cilium-x7k2p: Pod kube-system/cilium-x7k2p on node worker-02 is managed by DaemonSet cilium; kubectl drain refuses it without --ignore-daemonsets and never evicts it.
  Pod/kube-system/kube-proxy-worker-02: Pod kube-system/kube-proxy-worker-02 is a static pod mirrored from node worker-02; it cannot be evicted or rescheduled and stops with the node.
  Pod/longhorn-system/engine-image-ei-1a2b3c4d-8xk2q: Pod longhorn-system/engine-image-ei-1a2b3c4d-8xk2q on node worker-02 is managed by DaemonSet engine-image-ei-1a2b3c4d; kubectl drain refuses it without --ignore-daemonsets and never evicts it.
  Remediation (Pod/kube-system/cilium-x7k2p, Pod/longhorn-system/engine-image-ei-1a2b3c4d-8xk2q): Drain with --ignore-daemonsets; the DaemonSet pod stays until the node itself goes down.
  Remediation (Pod/kube-system/kube-proxy-worker-02): Plan for the static pod to be unavailable while the node is down; kubectl drain skips it.

This report describes the observed cluster state at 2026-10-15T20:06:57Z and does not guarantee execution success. Cluster state may drift immediately after analysis.
$ echo $?
1
```

The exit code is `1` here because a `BLOCKED` finding reaches the default `--fail-on blocked`. See [docs/USAGE.md](docs/USAGE.md) for every flag, rule and exit code.

## Install

Linux, macOS and Windows binaries (`amd64` and `arm64`) are on the [releases page](https://github.com/Veinar/draincheck/releases). Releases are signed with cosign (keyless) and ship SLSA provenance. Install and verification steps: [docs/RELEASE.md](docs/RELEASE.md).

## Build

```sh
go test ./...
go build -o draincheck ./cmd/draincheck
```

Requires Go 1.26+.

## Docs

[Specification](docs/SPEC.md) · [Rules and noise budget](docs/RULES.md) · [CI](docs/CI.md) · [Release and verification](docs/RELEASE.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[Apache-2.0](LICENSE)
