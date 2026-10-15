## What

<!-- One or two sentences: what changes and why. Link the issue: Closes #123 -->

## Checklist

- [ ] PR title follows Conventional Commits: `type(scope): subject`
- [ ] Commits are signed (`-S`) and signed off (`-s`)
- [ ] Tests added or updated
- [ ] One file = one concern; test file mirrors source file
- [ ] `docs/` updated if behaviour changed
- [ ] Analyzer rule added or changed: its entry in `docs/RULES.md` answers the noise-budget criteria
- [ ] `CHANGELOG.md` updated under `[Unreleased]` for user-visible changes (or label `skip-changelog`)
- [ ] `go vet ./...` and `go test ./...` pass
