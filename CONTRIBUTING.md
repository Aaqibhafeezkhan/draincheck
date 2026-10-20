# Contributing to draincheck

Thanks for taking the time to contribute. draincheck is a small, read-only Go CLI that preflight-checks a multi-node `kubectl drain`. Focused, well-tested changes are the most useful kind.

This file is the short version. The full engineering rules live in [`AGENTS.md`](AGENTS.md); the behaviour of the tool is defined in [`docs/SPEC.md`](docs/SPEC.md). Please read both before your first change.

## Ground rules

- **The tool never mutates the cluster.** List, get and watch only. No cordon, evict, patch or delete. A change that needs a write verb will not be accepted.
- **No `SAFE` status.** Findings are `BLOCKED`, `RISK` or `UNKNOWN`.
- **Be kind and constructive.** Assume good intent, review the code and not the person, and keep discussions on topic.

## Table of contents

- [Ways to contribute](#ways-to-contribute)
- [I have a question](#i-have-a-question)
- [Reporting bugs](#reporting-bugs)
- [Suggesting enhancements](#suggesting-enhancements)
- [Your first code contribution](#your-first-code-contribution)
- [Improving the documentation](#improving-the-documentation)
- [Security issues](#security-issues)

## Ways to contribute

All kinds of contributions are welcome, and not only code:

- **Pick an open issue.** Issues labeled [`good first issue`](https://github.com/Veinar/draincheck/labels/good%20first%20issue) are the best start; [`help wanted`](https://github.com/Veinar/draincheck/labels/help%20wanted) marks work the maintainer would like help with.
- **Report a bug** or **suggest an enhancement** (see below).
- **Improve the docs.** Typos, unclear wording and missing examples are all welcome.
- **Try it on a real cluster** and tell us what was confusing, missing or wrong in the findings.

Short on time? These help the project too:

- Star the repository.
- Mention draincheck in your own project's README, blog or team chat, or at a local meetup.

## I have a question

Please check the [README](README.md), [`docs/USAGE.md`](docs/USAGE.md) and [`docs/SPEC.md`](docs/SPEC.md) first, then search the existing [issues](https://github.com/Veinar/draincheck/issues) (open and closed). If you still need help, open an issue and include:

- what you are trying to do and what you ran (flags, redacted output);
- the output of `draincheck --version`, your OS, and your Kubernetes version.

## Reporting bugs

A good report saves a round trip. Before you file one:

- Make sure you run the latest `develop` or release.
- Check that it is a bug and not a misunderstanding; see the docs above.
- Search the existing issues for the same problem.

Then open an issue with:

- the command you ran, the output you got, and the output you expected;
- `draincheck --version`, OS and architecture, Go version if you built it yourself, and the Kubernetes server version;
- steps to reproduce. A minimal manifest, ideally in the shape of the `testdata/` fixtures, makes a fix much faster. **Redact** node, namespace and workload names you do not want to share.

## Suggesting enhancements

Before you open a suggestion, search the existing issues; if the idea is there, comment on it instead. Then check that it fits the project: draincheck is a **read-only** preflight analyzer, not a controller and not a scheduler simulator.

In the issue, describe the problem first, the current behaviour, the behaviour you would expect, and why it is useful to most users, not only to your setup. New or changed findings should say which severity they map to (`BLOCKED`, `RISK` or `UNKNOWN`).

## Your first code contribution

Start with a [`good first issue`](https://github.com/Veinar/draincheck/labels/good%20first%20issue), then follow [Development setup](#development-setup) and [Workflow](#workflow) below.

### Claim an issue first

Comment on the issue that you want to take it, and wait for a reply before you start. This avoids two people building the same thing. If you stop working on it, please say so, so someone else can pick it up.

## Security issues

Do not post details of a vulnerability in a public issue. A `SECURITY.md` with a reporting process is *planned*; until it exists, open an issue that says only that you have a security report, without technical details, and the maintainer will arrange a private channel.

## Development setup

Requirements: Go 1.26 or newer (see `go.mod`). No cluster is needed; unit tests use the client-go fake clientset and fixtures from `testdata/`.

```sh
git clone https://github.com/<your-user>/draincheck.git
cd draincheck
git remote add upstream https://github.com/Veinar/draincheck.git

go build ./...
go vet ./...
go test ./...
```

`make build`, `make test`, `make vet` and `make fmt` are shortcuts for the same commands.

## Workflow

1. **Fork** the repository and branch off `develop` (not `main`; the default branch is `develop`).
2. **One branch per change.** Name it `<type>/<scope>-<slug>`, mirroring the commit type and scope; the scope is optional.

   ```text
   feat/cli-nodes-flag      fix/findings-sort-order      docs/readme-usage
   ```

   Types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`, `chore`.
3. **Write the test first**, watch it fail, then make it pass.
4. **Keep the change small and focused.** One concern per pull request; do not mix refactoring with behaviour changes.
5. **Open a pull request against `develop`** and fill in the template. Link the issue with `Closes #<number>`.

### Commits and pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org/): `type(scope): subject`, for example `feat(analyzer): add PDB_DISRUPTION_BLOCKED rule`. The pull request title must follow the same format.
- **Every commit must be signed off and signed:**

  ```sh
  git commit -S -s -m "fix(findings): keep sort order stable"
  ```

  `-s` adds a `Signed-off-by:` line that must match your commit author e-mail. It certifies that you wrote the change, or have the right to submit it, under the project license ([Developer Certificate of Origin](https://developercertificate.org/)). `-S` signs the commit; add your signing key to your GitHub account so the commit shows as *Verified*.
- The `pr-validation.yml` workflow checks the title, the sign-off, the signature and the changelog entry. See [`docs/CI.md`](docs/CI.md) for every check and how to run them locally.

## Code conventions

The details are in `AGENTS.md`. The ones reviewers look at first:

- **One file, one concern.** One type or one rule per file; the test file mirrors the source file (`foo.go` -> `foo_test.go`).
- **Respect the layout.** `cmd/` stays thin (flags, wiring, exit codes); logic lives in `internal/`. `findings` imports nothing internal.
- **Typed evidence.** Concrete structs, never `map[string]interface{}`.
- **Deterministic output.** Sort findings with `findings.Sort` before printing or serializing.
- **`gofmt` and `go vet ./...` must be clean**, and `go test ./...` must pass.

## Improving the documentation

Docs ship with code. A change to a flag, rule, severity, output format, exit code or required permission must update `docs/` in the same pull request. Document only what is implemented; mark the rest *planned*.

## Updating the changelog

Add an entry under `## [Unreleased]` in `CHANGELOG.md` in the same pull request. The maintainer does not edit it at merge time.

- Needed for user-visible changes: `feat`, `fix`, `perf`, behaviour-changing refactors, dependency bumps that affect users, security fixes.
- Not needed for `docs`, `test`, `ci`, `chore` or internal refactors.
- Use the Keep a Changelog groups: Added, Changed, Deprecated, Removed, Fixed, Security. Analyzer rules go under `#### Analyzer rules`.
- Write for users, not as a copy of the commit subject, and link the issue or PR, e.g. `(#12)`.
- Credit is optional. If you want to be listed, add your GitHub handle to the entry, e.g. `(#12, @username)`. Leave it out if you prefer not to be named.
- CI fails a PR that changes non-test Go code in `cmd/` or `internal/` without touching `CHANGELOG.md`. A maintainer can apply the `skip-changelog` label when no entry is warranted.

## Pull request checklist

- [ ] Branch is based on `develop` and named `<type>/<scope>-<slug>`
- [ ] Title and commits follow Conventional Commits
- [ ] Every commit is signed (`-S`) and signed off (`-s`)
- [ ] I read and understand every line, and can explain it in review
- [ ] Tests were added or updated, and written first
- [ ] `gofmt -l .`, `go vet ./...` and `go test ./...` are clean
- [ ] `docs/` is updated if behaviour changed
- [ ] `CHANGELOG.md` has an entry under `[Unreleased]` for user-visible changes

## AI-assisted contributions

AI tools are fine. Vibe-coding is not. You are accountable for every line you submit: read it, run it, and be able to explain and defend it in review. The sign-off certifies that you did.

A contribution the author cannot explain is closed, whichever tool wrote it. "The AI wrote it" is not an answer to a review question. If a tool wrote a significant part, say so in the PR description; it helps the review.

## Legal notice

By contributing you confirm that you wrote the contribution or have the right to submit it, and that it may be provided under the [Apache License 2.0](LICENSE), the same as the project. The sign-off on each commit records this.
