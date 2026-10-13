# Security Policy

draincheck is a read-only preflight checker for `kubectl drain`. It lists and
reads cluster state and never cordons, evicts, patches or deletes anything.
Because people rely on its output to decide whether a drain is safe, we treat
both classic vulnerabilities and incorrect results as worth reporting.

## Supported versions

Security fixes are made for the latest released version and for the `develop`
branch. Older releases do not receive backports; please upgrade.

| Version                | Supported          |
| ---------------------- | ------------------ |
| Latest release         | :white_check_mark: |
| `develop`              | :white_check_mark: |
| Anything older         | :x:                |

## Reporting a vulnerability

**Do not open a public issue, pull request or discussion for a suspected
vulnerability.**

Report it privately through GitHub private vulnerability reporting:

<https://github.com/Veinar/draincheck/security/advisories/new>

If you cannot use GitHub, email conrad.kurek+draincheck@gmail.com with the
subject "draincheck security". Please do not include exploit details in public
channels. We will move the discussion into a private advisory.

Please include, as far as you can:

- the affected version or commit (`draincheck --version`, or a commit SHA);
- a description of the issue and its impact;
- steps to reproduce, ideally with a minimal manifest or fixture;
- any suggested fix or mitigation.

Do not include real cluster credentials, kubeconfigs, tokens or customer data
in a report. If a proof of concept needs a cluster, use a throwaway one.

## What to expect

| Step                                   | Target                              |
| -------------------------------------- | ----------------------------------- |
| Acknowledgement of your report         | within 3 business days              |
| Initial assessment and severity        | within 14 days                      |
| Status updates while a fix is prepared | at least every 14 days              |
| Fix released for confirmed issues      | as soon as practical, by severity   |

This is a volunteer-maintained project, so these are targets rather than
guarantees. If you hear nothing within the acknowledgement window, please
comment on your advisory to nudge us.

## Disclosure policy

We follow coordinated disclosure:

1. We confirm the issue and work on a fix privately, in the advisory.
2. We agree a disclosure date with the reporter, normally within 90 days of
   the report and sooner once a fix is available.
3. We publish a fixed release and a GitHub Security Advisory, requesting a CVE
   where appropriate, and credit the reporter unless they prefer to remain
   anonymous.

## Scope

In scope:

- code execution, privilege escalation or credential exposure through
  draincheck, its input handling (kubeconfig, flags, cluster objects) or its
  output formatting;
- any code path that writes to the cluster. draincheck must be strictly
  read-only, so a mutating API call is a vulnerability;
- requesting more RBAC permissions than documented in `docs/USAGE.md`;
- vulnerabilities in release artifacts or the build and release pipeline, such
  as unsigned or tampered artifacts;
- a finding or exit code that wrongly reports no problem for a condition the
  tool claims to detect, when the cause is a defect rather than a documented
  limitation.

Out of scope:

- vulnerabilities in Kubernetes, client-go or other dependencies themselves
  (report those upstream; we will update the dependency once a fix exists);
- results that follow from documented limitations, for example the *planned*
  rules or the explicit `UNKNOWN` outcomes;
- issues that require an already compromised cluster-admin account or a
  malicious local user with write access to the binary or kubeconfig;
- denial of service through a deliberately enormous cluster snapshot on a
  machine you control.

## Safe harbor

We will not pursue or support legal action against people who make a
good-faith effort to follow this policy: test only against systems you own or
have permission to test, avoid privacy violations and service disruption, and
give us reasonable time to respond before any public disclosure.

## Handling of dependencies and builds

- Dependencies are tracked by Dependabot and updated through reviewed pull
  requests.
- Changes land through pull requests with required status checks; commits by
  maintainers are signed and signed off.
- Contribution rules, including the read-only requirement, are in
  [AGENTS.md](AGENTS.md) and [CONTRIBUTING.md](CONTRIBUTING.md).
