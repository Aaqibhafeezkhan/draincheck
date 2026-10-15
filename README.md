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
draincheck --nodes worker-02,worker-04 [--kubeconfig PATH] [--context NAME] [--output text|json] [--verbose] [--fail-on blocked|risk|any]
```

| Flag           | Description                                    | Default                |
| -------------- | ---------------------------------------------- | ---------------------- |
| `--nodes`      | Nodes to drain, comma-separated, or `'*'` for all (required) | none     |
| `--kubeconfig` | Path to kubeconfig                             | standard loading rules |
| `--context`    | Kubeconfig context                             | current context        |
| `--output`     | Output format: `text` or `json`                | `text`                 |
| `--verbose`    | Text output only: print every finding in full  | off                    |
| `--fail-on`    | Lowest finding level that exits `1`: `blocked`, `risk` or `any` | `blocked` |

> [!TIP]
> You usually do not need `--kubeconfig` or `--context`. draincheck loads your kubeconfig with the same rules as `kubectl`: the `KUBECONFIG` environment variable if set, otherwise `~/.kube/config`, using the current context. If `kubectl get nodes` works for the user running draincheck, `draincheck --nodes worker-02` works too. It does not call the `kubectl` binary; it reads the same kubeconfig and talks to the API server directly, so `kubectl` does not have to be installed.

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
