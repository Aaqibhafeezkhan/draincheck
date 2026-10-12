# CI

GitHub Actions in `.github/workflows/`. Actions are pinned by commit SHA.

## Build and quality

| Workflow          | Runs on                | What                                                                 |
| ----------------- | ---------------------- | -------------------------------------------------------------------- |
| `ci.yml`          | push, PR to `develop`  | `go build`, `go vet`, `gofmt -l`, `go mod tidy` clean, `go test -race`, `golangci-lint` |
| `pr-validation.yml` | PR to `develop`      | title is `type(scope): subject`; every commit is signed off (`-s`) and Verified (`-S`); coverage diff (informational) |
| `workflow-lint.yml` | PR touching workflows | `actionlint`                                                         |

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
gofmt -l .
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
BASE_SHA=$(git merge-base develop HEAD) bash .github/scripts/coverage-diff.sh
```
