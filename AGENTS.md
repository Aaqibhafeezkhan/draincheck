# AGENTS.md

Instructions for AI coding assistants working on draincheck.

## Project

Read-only Go CLI that preflight-checks a multi-node `kubectl drain`. Spec: `docs/SPEC.md`. Module: `github.com/Veinar/draincheck`.

## Hard rules

- **Never mutate the cluster.** List/get/watch only. No cordon, evict, patch or delete.
- **No `SAFE` status.** Findings are `BLOCKED`, `RISK` or `UNKNOWN` only.
- **Typed evidence.** Concrete structs, never `map[string]interface{}`.
- **Deterministic output.** Sort findings with `findings.Sort` before printing or serializing.
- **Account for every action.** Each cluster read, rule run and decision that shapes the result writes a `DEBUG` line (`--debug`, stderr only). Never log secrets. Details under Code conventions.
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
- **Debug accounts for actions.** `--debug` is how an operator checks what draincheck did, so every action has a `DEBUG` line: each cluster read (resource, count, duration), each rule (start, finish, findings) and each decision that shapes the result (target nodes, severity counts, exit code). A new API call, rule or decision ships with its `DEBUG` line and a test for it, and its row in the Debug output table in `docs/USAGE.md`. Log what was done and the outcome, not object contents. Never log tokens, certificates, `Authorization` headers or kubeconfig contents; log paths and names only. Use the `*slog.Logger` passed in from `cmd/draincheck` (a nil logger discards); no package-level logger. Diagnostics go to stderr only, and stdout must be byte-identical with and without `--debug`.
- Test first (red, then green). Unit tests use the client-go fake clientset and `testdata/`.
- `gofmt` and `go vet ./...` must be clean.
- **Docs ship with code.** A change to a flag, rule, severity, output format, exit code or required permission updates `docs/` in the same change. Document only what is implemented; mark the rest *planned*.
- **Changelog ships with code.** A user-visible change (feat, fix, perf, a behavior-changing refactor, a dependency bump that affects users, a security fix) adds an entry under `## [Unreleased]` in `CHANGELOG.md` in the same change. Follow Keep a Changelog groups (Added, Changed, Deprecated, Removed, Fixed, Security); put analyzer rules under `#### Analyzer rules`. Write for users, not as a copy of the commit subject; link the issue or PR, e.g. `(#12)`. **Ask the contributor** whether they want to be credited in the changelog; add their GitHub handle, e.g. `(#12, @username)`, only if they say yes. Never assume consent or guess a handle. Skip `docs`, `test`, `ci`, `chore` and internal refactors. CI enforces this for non-test Go code in `cmd/` and `internal/` (bypass: `skip-changelog` label). The maintainer does not edit the changelog at merge time.

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
