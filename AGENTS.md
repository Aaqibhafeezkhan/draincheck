# AGENTS.md

Instructions for AI coding assistants working on draincheck.

## Project

Read-only Go CLI that preflight-checks a multi-node `kubectl drain`. Spec: `docs/SPEC.md`. Module: `github.com/Veinar/draincheck`.

## Hard rules

- **Never mutate the cluster.** List/get/watch only. No cordon, evict, patch or delete.
- **No `SAFE` status.** Findings are `BLOCKED`, `RISK` or `UNKNOWN` only.
- **Typed evidence.** Concrete structs, never `map[string]interface{}`.
- **Deterministic output.** Sort findings with `findings.Sort` before printing or serializing.
- **Ask, don't guess.** Open questions go to the user.

## Layout

```text
cmd/draincheck/     CLI only: flags, wiring, exit codes
internal/inventory/ read-only snapshot of cluster state
internal/analyzer/  pure, stateless rules
internal/findings/  typed findings and evidence
internal/report/    text and JSON output
testdata/           fixture YAML
tests/              integration tests
```

Dependency direction: `cmd` -> `inventory`, `analyzer`, `report` -> `findings`. `findings` imports nothing internal.

## Code conventions

- **One file = one concern.** One type or one rule per file; no grab-bag files.
- **Test file mirrors source file:** `foo.go` -> `foo_test.go`.
- Keep `cmd/` thin; logic lives in `internal/`.
- Test first (red, then green). Unit tests use the client-go fake clientset and `testdata/`.
- `gofmt` and `go vet ./...` must be clean.

## Workflow

- Branch off `develop`, one branch per change. Branch name is `<type>/<scope>-<slug>`, mirroring the commit `type(scope)`; the scope is optional (`<type>/<slug>`).
- Prefixes: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`, `chore`.
- Examples: `feat/cli-nodes-flag` (commit `feat(cli): ...`), `fix/findings-sort-order`, `docs/readme-usage`, `chore/makefile`.
- Commits follow Conventional Commits: `type(scope): subject`, e.g. `fix(cli): ...`, `feat(analyzer): ...`, `chore: ...`, `docs: ...`.
- **Never commit.** Leave changes staged only. Do not run `git commit`, `git push`, or open PRs.
- **Always tell the operator** that they must review the staged changes manually and run the commit themselves. Include a suggested message.
- The operator commits signed and signed off: `git commit -S -s`.

## Commands

```sh
go build ./...
go vet ./...
go test ./...
```
