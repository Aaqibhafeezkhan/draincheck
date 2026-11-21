package analyzer

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
)

// AddVerify returns a copy of fs with Verify filled in for every finding, so a
// rule does not set it. It does not sort or drop findings and leaves fs alone.
// It writes one DEBUG line to logger with the number of findings that got
// commands and the number of commands; a nil logger discards it.
func AddVerify(logger *slog.Logger, fs []findings.Finding) []findings.Finding {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	out := slices.Clone(fs)
	withCommands, commands := 0, 0
	for i := range out {
		out[i].Verify = verifyCommands(out[i])
		if n := len(out[i].Verify); n > 0 {
			withCommands++
			commands += n
		}
	}
	logger.Debug("verify commands", "findings", withCommands, "commands", commands)
	return out
}

// verifyCommands returns the read-only kubectl commands that confirm f, with
// the names of f filled in, or nil when f is of a kind it does not know. They
// are advice for the operator: nothing in draincheck runs them. A command that
// needs a pod or node name the evidence does not hold is left out, so none has
// a hole in it. docs/RULES.md ("Advice standard") lists them per rule, and
// TestAdviceContract holds them to the same read-only rules as the remediation.
func verifyCommands(f findings.Finding) []string {
	r := f.Resource
	switch ev := f.Evidence.(type) {
	case findings.PDBEvidence, findings.PDBUnsatisfiableEvidence:
		return []string{
			fmt.Sprintf("kubectl get pdb -n %s %s", r.Namespace, r.Name),
			fmt.Sprintf("kubectl get pdb -n %s %s -o yaml", r.Namespace, r.Name),
		}
	case findings.PDBBatchEvidence:
		return []string{fmt.Sprintf("kubectl get pdb -n %s %s", r.Namespace, r.Name)}
	case findings.PDBStaleStatusEvidence:
		return []string{fmt.Sprintf("kubectl get pdb -n %s %s -o jsonpath='{.metadata.generation} {.status.observedGeneration}'", r.Namespace, r.Name)}
	case findings.PDBSyncFailedEvidence:
		return []string{fmt.Sprintf("kubectl get pdb -n %s %s -o jsonpath='{.status.conditions}'", r.Namespace, r.Name)}
	case findings.OperatorManagedPDBsEvidence:
		cmds := []string{fmt.Sprintf("kubectl get pdb -n %s", ev.Namespace)}
		if len(ev.PDBs) > 0 {
			cmds = append(cmds, fmt.Sprintf("kubectl get pdb -n %s %s -o yaml", ev.Namespace, ev.PDBs[0].Name))
		}
		return cmds
	case findings.OverlappingPDBsEvidence:
		cmds := []string{fmt.Sprintf("kubectl get pod -n %s %s --show-labels", r.Namespace, r.Name)}
		for _, name := range ev.PDBs {
			cmds = append(cmds, fmt.Sprintf("kubectl get pdb -n %s %s -o yaml", ev.Namespace, name))
		}
		return cmds
	case findings.NodeNotSchedulableEvidence:
		if len(ev.Nodes) == 0 {
			return nil
		}
		names := make([]string, len(ev.Nodes))
		for i, n := range ev.Nodes {
			names[i] = n.Name
		}
		return []string{
			"kubectl get node " + strings.Join(names, " "),
			"kubectl describe node " + names[0],
		}
	case findings.NoRemainingNodesEvidence:
		return []string{
			"kubectl get nodes",
			"kubectl get pods -A --field-selector=status.phase=Pending",
		}
	case findings.WorkloadUntoleratedTaintsEvidence:
		var cmds []string
		if len(ev.TaintedNodes) > 0 {
			cmds = append(cmds, "kubectl describe node "+ev.TaintedNodes[0].Name)
		}
		return append(cmds, podCommands(r.Namespace, strandedName(ev.Pods), "{.spec.tolerations}")...)
	case findings.WorkloadUnmatchedLabelsEvidence:
		return append([]string{"kubectl get nodes --show-labels"},
			podCommands(r.Namespace, strandedName(ev.Pods), "{.spec.nodeSelector}", "{.spec.affinity.nodeAffinity}")...)
	case findings.WorkloadPlacementCombinedEvidence:
		var cmds []string
		if len(ev.RuledOutNodes) > 0 {
			cmds = append(cmds, "kubectl describe node "+ev.RuledOutNodes[0].Name)
		}
		cmds = append(cmds, "kubectl get nodes --show-labels")
		return append(cmds, podCommands(r.Namespace, strandedName(ev.Pods), "{.spec.tolerations}")...)
	case findings.SingleReplicaEvidence:
		return []string{
			fmt.Sprintf("kubectl get %s -n %s %s -o jsonpath='{.spec.replicas}'", r.Kind, r.Namespace, r.Name),
			fmt.Sprintf("kubectl get pdb -n %s", r.Namespace),
		}
	case findings.AllReplicasEvidence:
		return []string{
			fmt.Sprintf("kubectl get pods -n %s -o wide", r.Namespace),
			fmt.Sprintf("kubectl get pdb -n %s", r.Namespace),
		}
	case findings.OwnershipEvidence:
		return ownershipCommands(r, ev)
	case findings.LocalStorageEvidence:
		if len(ev.Pods) == 0 {
			return nil
		}
		return podCommands(ev.Pods[0].Namespace, ev.Pods[0].Name, "{.spec.volumes}")
	case findings.MissingRequestsEvidence:
		if len(ev.Pods) == 0 {
			return nil
		}
		return podCommands(r.Namespace, ev.Pods[0].Name, "{.spec.containers[*].resources.requests}")
	case findings.StuckTerminatingEvidence:
		return []string{
			fmt.Sprintf("kubectl get pod -n %s %s -o jsonpath='{.metadata.finalizers}'", r.Namespace, r.Name),
			"kubectl get node " + ev.Node,
		}
	}
	return nil
}

// ownershipCommands returns the commands of an ownership finding, by category.
func ownershipCommands(r findings.ResourceRef, ev findings.OwnershipEvidence) []string {
	switch ev.Category {
	case findings.OwnershipDaemonSet, findings.OwnershipOrphanedDaemonSet:
		return []string{fmt.Sprintf("kubectl get daemonset -n %s %s", r.Namespace, r.Name)}
	case findings.OwnershipStaticPod:
		if len(ev.Nodes) == 0 || len(ev.Pods) == 0 {
			return nil
		}
		p := ev.Pods[0]
		return []string{
			"kubectl get pods -A --field-selector spec.nodeName=" + ev.Nodes[0],
			fmt.Sprintf("kubectl get pod -n %s %s -o jsonpath='{.metadata.annotations}'", p.Namespace, p.Name),
		}
	case findings.OwnershipUnmanaged:
		return []string{fmt.Sprintf("kubectl get pod -n %s %s -o jsonpath='{.metadata.ownerReferences}'", r.Namespace, r.Name)}
	}
	return nil
}

// podCommands returns one `kubectl get pod` per jsonpath for the named pod, or
// nil when there is no pod to name.
func podCommands(namespace, pod string, jsonpaths ...string) []string {
	if pod == "" {
		return nil
	}
	cmds := make([]string, len(jsonpaths))
	for i, jp := range jsonpaths {
		cmds[i] = fmt.Sprintf("kubectl get pod -n %s %s -o jsonpath='%s'", namespace, pod, jp)
	}
	return cmds
}

// strandedName returns the name of the first pod, or "" for none.
func strandedName(pods []findings.StrandedPod) string {
	if len(pods) == 0 {
		return ""
	}
	return pods[0].Name
}
