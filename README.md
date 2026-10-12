# draincheck

![GitHub Created At](https://img.shields.io/github/created-at/Veinar/draincheck?color=%234ecca3)
![GitHub last commit](https://img.shields.io/github/last-commit/Veinar/draincheck)
[![Go](https://img.shields.io/badge/Go-%2300ADD8.svg?&logo=go&logoColor=white)](#)
![GitHub top language](https://img.shields.io/github/languages/top/Veinar/draincheck)
![GitHub License](https://img.shields.io/github/license/Veinar/draincheck?color=%23dc9c1d)
![GitHub Tag](https://img.shields.io/github/v/tag/Veinar/draincheck)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/Veinar/draincheck?color=%233c5c68)
![Contrib welcome](https://img.shields.io/badge/contributions-welcome-blue)

[![CI](https://github.com/Veinar/draincheck/actions/workflows/ci.yml/badge.svg?branch=develop)](https://github.com/Veinar/draincheck/actions/workflows/ci.yml)
[![CodeQL](https://github.com/Veinar/draincheck/actions/workflows/codeql.yml/badge.svg?branch=develop)](https://github.com/Veinar/draincheck/actions/workflows/codeql.yml)
[![govulncheck](https://github.com/Veinar/draincheck/actions/workflows/govulncheck.yml/badge.svg?branch=develop)](https://github.com/Veinar/draincheck/actions/workflows/govulncheck.yml)
[![OpenSSF Scorecard](https://github.com/Veinar/draincheck/actions/workflows/scorecard.yml/badge.svg?branch=develop)](https://github.com/Veinar/draincheck/actions/workflows/scorecard.yml)

[![Claude](https://img.shields.io/badge/Claude-D97757?logo=claude&logoColor=fff)](#)

> [!NOTE]
> **AI-assisted, not vibe-coded.** AI helps with code; every change is reviewed and committed by a human.

Read-only preflight analyzer for multi-node `kubectl drain`. Tells you what would block or put at risk a planned maintenance **before** you cordon anything.

> **Status: pre-alpha.** Only the CLI flags are implemented; no analysis yet.

## What it is

- A read-only Go CLI built on `client-go`. It never cordons, evicts or modifies anything.
- Findings are `BLOCKED`, `RISK` or `UNKNOWN`. There is deliberately no `SAFE`.

## What it is not

- Not a controller.
- Not a scheduler simulator.

## Usage

```sh
draincheck --nodes worker-02,worker-04 [--kubeconfig PATH] [--context NAME] [--output text|json]
```

| Flag           | Description                                    | Default                |
| -------------- | ---------------------------------------------- | ---------------------- |
| `--nodes`      | Nodes to drain, comma-separated, or `'*'` for all (required) | none     |
| `--kubeconfig` | Path to kubeconfig                             | standard loading rules |
| `--context`    | Kubeconfig context                             | current context        |
| `--output`     | Output format: `text` or `json`                | `text`                 |

## Build

```sh
go build -o draincheck ./cmd/draincheck
go test ./...
```

Requires Go 1.26+.

## Docs

[Specification](docs/SPEC.md)

## License

[Apache-2.0](LICENSE)
