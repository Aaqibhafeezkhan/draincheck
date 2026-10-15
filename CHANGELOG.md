# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Each entry should link to the pull request or issue that introduced it. Contributors who opt in are credited with their GitHub handle, e.g. `(#12, @username)`. Add new
entries under `[Unreleased]` in the same pull request as the change (see
[CONTRIBUTING.md](CONTRIBUTING.md)).

## [Unreleased]

First development cycle. No version has been tagged yet.

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
- `POD_OWNERSHIP`: DaemonSet, static and naked pods (#5).
- `MISSING_RESOURCE_REQUESTS`: pods without CPU or memory requests (#7).

### Changed

- Bump Go to 1.26 and refresh indirect dependencies.
- Bump `k8s.io/apimachinery` and `k8s.io/client-go` from 0.35.9 to 0.37.1.

[Unreleased]: https://github.com/Veinar/draincheck/commits/develop
