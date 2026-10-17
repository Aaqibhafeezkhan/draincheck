# draincheck rules

Design rules for analyzer rules. Every rule that can raise `BLOCKED` or `RISK` must say up front how it avoids crying wolf. A false `BLOCKED` costs more trust than a missed `RISK`, so this is a design requirement, not polish.

What each rule reports is described in [USAGE.md](USAGE.md#rules). This file records how each rule keeps its noise down.

## Noise budget

Every rule answers the eight criteria below, in its entry in this file and in the pull request that adds or changes it.

| # | Criterion | Requirement |
| - | --------- | ----------- |
| 1 | False-positive case | The realistic situation in which the rule fires although the drain would succeed. |
| 2 | Downgrade | How that case is detected and what severity it gets instead. `UNKNOWN` when state is stale or a constraint cannot be evaluated; never keep `BLOCKED` on doubt. |
| 3 | Mirror kubectl | A rule must not tell the operator to fix something `kubectl drain` handles on its own. For pods kubectl skips (DaemonSet-managed with `--ignore-daemonsets`, mirror pods, terminal pods), a rule whose rationale is eviction or rescheduling emits nothing, and its entry says why. Information that is still true (a DaemonSet needs the flag, static pods go down with the node) is reported once, grouped, not per pod. |
| 4 | Grouping | How many findings a large workload produces (10 replicas behind one PDB) and whether findings are grouped, with per-pod detail kept in typed evidence. One finding per cause, never one per pod. |
| 5 | Estimates | A rule based on an estimate (capacity, soft preferences) never emits `BLOCKED`, shows the numbers it used and states that it is an estimate. |
| 6 | Evidence | The finding carries enough typed evidence to verify it in seconds. |
| 7 | Fixture | `testdata/` includes the false-positive case and a test proving the downgrade. |
| 8 | Visibility | Nothing is dropped silently. Every fact a rule used to report per pod stays reachable, in grouped typed evidence (JSON) or in verbose text. Reducing noise changes how a fact is presented, not whether it exists. |

### Template

Copy this into the entry of a new rule.

```markdown
### `RULE_ID`

Severity: `BLOCKED` | `RISK` | `UNKNOWN`.

| Criterion | Answer |
| --- | --- |
| False-positive case | |
| Downgrade | |
| Mirror kubectl | |
| Grouping | |
| Estimates | |
| Evidence | |
| Fixture | |
| Visibility | |
```

Mark anything not implemented yet as *planned* and link the issue. Document only what the code does.

## Entries

### `PDB_DISRUPTION_BLOCKED`

Severity: `BLOCKED`.

| Criterion | Answer |
| --- | --- |
| False-positive case | `status.disruptionsAllowed` is read as is. A PDB whose status is not yet reconciled (`observedGeneration` differs from `generation`) reports a stale number: *planned*, see *PDB_STALE_STATUS and downgrade BLOCKED to UNKNOWN on unreconciled PDB*. A PDB owned by an operator that removes it when the node is cordoned is not told apart from a plain one: *planned*, see *report PDBs of operator-managed pods as UNKNOWN, not BLOCKED*. A PDB that selects a DaemonSet or static pod is reported although kubectl never evicts those pods: no downgrade yet, planned for later together with the one-finding-per-PDB change. |
| Downgrade | Implemented: pods in phase `Succeeded` or `Failed` are skipped, and a PDB with an invalid selector is skipped. Nothing else is downgraded yet; the stale and operator-managed cases above become `UNKNOWN` when implemented. |
| Mirror kubectl | Terminal pods only, see above. DaemonSet and mirror pods are *not* skipped yet. |
| Grouping | None: one finding per pod and PDB, so 10 replicas behind one PDB give 10 findings. One finding per PDB is *planned*, see *PDB_DISRUPTION_BLOCKED must emit one finding per PDB*. |
| Estimates | Not an estimate: the number comes from the PDB status. |
| Evidence | `findings.PDBEvidence`: PDB namespace and name, `current_healthy`, `desired_healthy`, `disruptions_allowed`. |
| Fixture | `testdata/analyzer`; the stale and operator-managed fixtures come with their issues. |
| Visibility | One finding per affected pod; nothing is hidden. |

### `PDB_OVERLAPPING_SELECTORS`

Severity: `BLOCKED`.

| Criterion | Answer |
| --- | --- |
| False-positive case | The eviction API rejects a pod selected by several PDBs whatever the budgets allow, so for a pod that is evicted the finding is exact. It is a false positive for a DaemonSet or static pod, which kubectl never evicts. |
| Downgrade | Implemented: terminal pods and PDBs with an invalid selector are skipped. No other downgrade; none is needed for evicted pods. |
| Mirror kubectl | Terminal pods only. DaemonSet and mirror pods are *not* skipped yet, which is planned for later together with the one-finding-per-PDB change. |
| Grouping | One finding per pod, with every matching PDB in evidence. Replicas sharing the same overlap each get a finding. |
| Estimates | Not an estimate. |
| Evidence | `findings.OverlappingPDBsEvidence`: `namespace` and the `pdbs` that select the pod, sorted by name. |
| Fixture | `testdata/analyzer`. |
| Visibility | One finding per affected pod; nothing is hidden. |

### `POD_OWNERSHIP`

Severity: `RISK`.

| Criterion | Answer |
| --- | --- |
| False-positive case | The facts are true, but kubectl handles DaemonSet and static pods on its own, so the finding is information, not a fault. One corner is not covered: a pod whose DaemonSet has been deleted is still reported as a DaemonSet pod, because draincheck does not list DaemonSets. kubectl treats that orphan differently (it stops with a not-found error and only deletes the pod with `--force`). Handling it is *planned*, see *POD_OWNERSHIP must handle orphaned DaemonSet pods (DaemonSet no longer exists)*. |
| Downgrade | Never `BLOCKED`: whether the drain stops depends on kubectl flags draincheck cannot observe, so every category is `RISK`. |
| Mirror kubectl | Terminal pods are skipped. DaemonSet and static pods are reported on purpose, because the operator needs the `--ignore-daemonsets` flag and must plan for the node outage; the remediation says kubectl skips them. Because that advice is the same for every pod of a kind, it is given once per finding, not once per pod. |
| Grouping | A DaemonSet is one finding, however many nodes it runs on (28 pods on 28 nodes is one finding). Static pods are one finding per node, listing the static pods that stop with it. A pod with no controller is one finding of its own, since each one is lost on drain. Collapsing static pods across nodes is left to the compact text report: the node is the unit of action, because `--nodes` picks nodes. |
| Estimates | Not an estimate. |
| Evidence | `findings.OwnershipEvidence`: `category`, `nodes` and `pods` (each with `namespace`, `name` and `node`), plus `owner_kind` and `owner_name` for a DaemonSet. |
| Fixture | `testdata/analyzer/pod-ownership.yaml`: a DaemonSet on three nodes, two DaemonSets with the same name in different namespaces, static pods on two nodes (one node with pods from two namespaces), naked pods, and managed and finished pods that must not be reported. |
| Visibility | Every pod stays in `pods` in the JSON report, with its namespace and node. The text report shows one line per finding; `--verbose` prints the full description, which names the static pods. |

### `MISSING_RESOURCE_REQUESTS`

Severity: `RISK`.

| Criterion | Answer |
| --- | --- |
| False-positive case | Requests are read from the stored pod spec, so values from a LimitRange or an admission webhook count as set. A pod that is not rescheduled is not affected; those are skipped (below). A pod with no controller is reported as its own workload, although nothing recreates it after a drain; the finding only matters if the operator recreates it elsewhere. |
| Downgrade | Estimate-class rule: it is only ever `RISK`. |
| Mirror kubectl | Implemented: terminal pods, DaemonSet pods and static (mirror) pods are skipped, because the rule's rationale (the pod may not fit where it is rescheduled) does not apply to pods that are never rescheduled. They stay visible through `POD_OWNERSHIP`. |
| Grouping | Findings are grouped per owning workload (Deployment, StatefulSet, Job and so on), found with `inventory.OwnerResolver`. A pod with no controller counts as its own workload. Ten replicas missing requests give one finding. The description states the count once (for example "2 of 3 pods of Deployment shop/web") and the per-pod detail goes into the evidence. |
| Estimates | Yes. The finding lists the containers and which request each lacks. Init containers are not inspected. |
| Evidence | `findings.MissingRequestsEvidence`: `owner_kind` and `owner_name` (omitted for a pod with no controller), `pods_on_targets` (the workload's pods on the target nodes that kubectl would evict, the denominator of the count) and `pods`. Each entry of `pods` has `name`, `node` and `containers` with `missing_cpu` and `missing_memory`. |
| Fixture | `testdata/analyzer/missing-resource-requests.yaml`: replicas of a Deployment on several nodes, a partly affected Deployment, a Job with a finished pod, DaemonSet and mirror pods. |
| Visibility | Every affected pod, its node and its containers are in `pods` in the JSON report. The text report shows the count; `--verbose` shows the full description. |

## See also

- [USAGE.md](USAGE.md#rules): what each rule reports.
- [SPEC.md](SPEC.md): the specification.
