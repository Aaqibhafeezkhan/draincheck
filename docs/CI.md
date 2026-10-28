# CI

GitHub Actions in `.github/workflows/`. Actions are pinned by commit SHA; Dependabot keeps them current.

## Build and quality

| Workflow          | Runs on                | What                                                                 |
| ----------------- | ---------------------- | -------------------------------------------------------------------- |
| `ci.yml`          | push, PR to `develop`  | `go build`, `go vet`, `gofmt -l`, `go mod tidy` clean, `go test -race`, `golangci-lint` |
| `pr-validation.yml` | PR to `develop`      | title is `type(scope): subject`; every commit is signed off (`-s`) and Verified (`-S`); `CHANGELOG.md` touched when non-test Go code in `cmd/` or `internal/` changes (bypass: `skip-changelog` label); coverage diff (informational) |
| `workflow-lint.yml` | PR touching workflows | `actionlint`                                                         |

## Security

| Workflow                | Runs on                      | What                                                       |
| ----------------------- | ---------------------------- | ---------------------------------------------------------- |
| `govulncheck.yml`       | push, PR, weekly             | known vulnerabilities, on the newest patched Go toolchain  |
| `codeql.yml`            | push, PR, weekly             | CodeQL static analysis                                     |
| `dependency-review.yml` | PR                           | fails on new dependencies with `high` severity issues      |
| `scorecard.yml`         | push, weekly                 | OpenSSF Scorecard, results in the Security tab             |
| `dependabot.yml`        | weekly                       | `gomod` (k8s.io/* grouped) and `github-actions` updates    |

CodeQL and Scorecard upload to the Security tab, which needs a public repository.

## Release

| Workflow      | Runs on                     | What                                                                                             |
| ------------- | --------------------------- | ------------------------------------------------------------------------------------------------ |
| `release.yml` | push of tag `vX.Y.Z`    | release notes from `CHANGELOG.md`, GoReleaser builds Linux, macOS and Windows (`amd64`/`arm64`), checksums, cosign keyless signature, SLSA provenance (`*.intoto.jsonl`) |

Top-level `permissions: read-all`; write permissions only on the `release`, `provenance` and `publish` jobs. The release is created as a draft and published by the last job after the provenance is attached (needed for release immutability). Config: `.goreleaser.yaml`. Details and verification: `docs/RELEASE.md`.

## Branch rules

`develop` is the only protected branch. The ruleset "Protected develop" targets the default branch and is active:

| Rule | Setting |
| ---- | ------- |
| Restrict deletions | on |
| Block force pushes | on |
| Require signed commits | on |
| Require a pull request before merging | 1 approval; the approval of the most recent reviewable push is required; an additional approval for unattributed Copilot changes; stale approvals are dismissed when new commits are pushed; code owner review and thread resolution are not required; merge methods: merge and squash |
| Require status checks to pass | `Build, vet and test`, `golangci-lint`, `PR title follows Conventional Commits`, `Commits are signed off and verified` (all from GitHub Actions); the branch must be up to date before merging |

The bypass list holds the repository admin role, for pull requests only: the maintainer works alone and merges their own pull request, but still cannot push to `develop` directly or skip the pull request. The other workflows (CodeQL, govulncheck, dependency review, Scorecard, `actionlint`) run on pull requests but are not required checks. There is no ruleset for tags. If a job in `ci.yml` or `pr-validation.yml` is renamed, update the required check name in the ruleset in the same change, or the pull request waits for a check that never reports.

## Coverage diff

`pr-validation.yml` job `coverage` runs `go test -coverprofile` on the PR base and on the PR, and writes a per-package table to the job summary. It is **informational only**: it never fails a PR and uses no third-party service. Logic: `.github/scripts/coverage-diff.sh`.

## PR validation rules

- Merge commits and `[bot]` authors are skipped.
- `Signed-off-by:` must match the commit author e-mail.
- Verified means GitHub shows the commit as Verified: add your signing key to your account.
- Logic: `.github/scripts/check-commits.sh`.

## Run locally

```sh
make vet test
make fuzz FUZZTIME=30s   # the Fuzz* targets; CI and `go test` only replay their seeds
gofmt -l .
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
BASE_SHA=$(git merge-base develop HEAD) bash .github/scripts/coverage-diff.sh
```
