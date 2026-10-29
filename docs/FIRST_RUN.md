# First run

Two things surprise a new reader. Both are expected. For the flags, rules and exit codes in full, see [USAGE.md](USAGE.md).

> [!NOTE]
> draincheck is read-only: it lists cluster state and never cordons, evicts or changes anything. The report describes a snapshot taken when you ran it, so run it again right before you drain.

1. **Pass the flags you will give `kubectl drain`.** draincheck judges the plan the way `kubectl drain` with the same flags would. Without `--ignore-daemonsets`, `--delete-emptydir-data` and `--force`, DaemonSet pods, pods with an `emptyDir` volume and pods with no controller on a target node are `BLOCKED`, because `kubectl drain` refuses them. A cluster with DaemonSets therefore exits `1` on the first run. The report says which ones in a `KUBECTL DRAIN FLAGS` section, right under the summary. Run again with the flags: that is the fix, not a finding to chase.
2. **Know what exit `0` covers.** The default `--fail-on blocked` looks only at `BLOCKED`. `RISK` and `UNKNOWN` findings are still printed and still matter. To gate a pipeline on `RISK` as well, use `--fail-on risk`. The exit codes are in [USAGE.md](USAGE.md#exit-codes).

> [!WARNING]
> Exit `0` is not a statement that the drain is safe. No `SAFE` status exists. In the second run below the exit code is `0` while two `RISK` findings remain.

The same invented cluster (`node-a`, `node-b` drained, `node-c` left; a DaemonSet `agent` and a pod of `shop/api` with an `emptyDir` volume and no requests), without and with the flags. Output of the current build; the long `Remediation` lines are left out here.

```text
$ draincheck --nodes node-a,node-b
=== FINDINGS SUMMARY ===
  BLOCKED: 2
  RISK:    1
  UNKNOWN: 0

=== KUBECTL DRAIN FLAGS ===
kubectl drain will refuse without: --ignore-daemonsets --delete-emptydir-data
(derived from the BLOCKED findings below, not a guarantee: run again with these flags to see what is left)

[POD_LOCAL_STORAGE] 1 finding
  ReplicaSet/shop/api-abc: ReplicaSet shop/api-abc has 1 pod with emptyDir volumes on 1 target node. kubectl drain refuses them without --delete-emptydir-data (`cannot delete Pods with local storage`) and stops before it evicts anything; waiting does not change that.
[POD_OWNERSHIP] 1 finding
  DaemonSet/monitoring/agent: DaemonSet monitoring/agent has 2 pods on 2 target nodes. kubectl drain refuses DaemonSet pods without --ignore-daemonsets and never evicts them.
[MISSING_RESOURCE_REQUESTS] 1 finding
  ...
$ echo $?
1

$ draincheck --nodes node-a,node-b --ignore-daemonsets --delete-emptydir-data
=== FINDINGS SUMMARY ===
  BLOCKED: 0
  RISK:    2
  UNKNOWN: 0

[MISSING_RESOURCE_REQUESTS] 1 finding
  ...
[POD_LOCAL_STORAGE] 1 finding
  ReplicaSet/shop/api-abc: ReplicaSet shop/api-abc has 1 pod with emptyDir volumes on 1 target node. With --delete-emptydir-data kubectl drain deletes the pods and the data in those volumes is lost; a controller recreates the pod with an empty volume.
$ echo $?
0
```

With the flags, `BLOCKED` is gone and the exit code is `0`, yet two `RISK` findings remain. Read them before you drain. What each rule means and where `kubectl drain` is ahead: [COMPARISON.md](COMPARISON.md).

> [!TIP]
> To see every finding in full, with its remediation, add `--verbose`. If a result surprises you, add `--debug`: it writes one line to stderr for each action (API calls, rules, the exit code) and leaves the report on stdout unchanged. See [Debug output](USAGE.md#debug-output).

> [!TIP]
> In a pipeline, `--fail-on risk` makes `RISK` findings exit `1` too, and `--fail-on any` includes `UNKNOWN`. Pass the same drain flags there as in the real drain.
