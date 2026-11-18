package analyzer

import (
	"cmp"
	"reflect"
	"slices"

	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
)

// placementReasonKind says what rules a node out for a workload's pods.
// Taints and labels are the kinds today; a new kind is a new constant and a new
// branch in placementOn, never a change to the rules that read the result.
type placementReasonKind string

const (
	// placementReasonTaint is a NoSchedule or NoExecute taint no pod tolerates.
	placementReasonTaint placementReasonKind = "taint"
	// placementReasonLabel is a nodeSelector entry or required node affinity
	// term the node's labels do not meet.
	placementReasonLabel placementReasonKind = "label"
)

// placementReason is one thing that rules a node out. Only the field of its
// Kind is set.
type placementReason struct {
	Kind  placementReasonKind
	Taint findings.NodeTaint
	Label findings.LabelMismatch
}

// nodePlacement is the answer of placementOn: whether any of the pods can land
// on the node and, when none can, every reason the node is ruled out.
//
// Room is the whole answer. TaintRoom and LabelRoom are the same question with
// one cause looked at alone: some pod has no untolerated taint on the node, or
// some pod has no unmet label requirement on it. A rule that states one cause
// reads its own field and the other's, so that the rules never report the same
// workload.
type nodePlacement struct {
	Room      bool
	TaintRoom bool
	LabelRoom bool
	Reasons   []placementReason
}

// placementOn reports whether any of the pods can land on node. When none can,
// the result lists the reasons: taints first, distinct and in the order the
// node lists them, then label requirements, distinct and sorted. With no pods
// there is nothing to land: no room of any kind, and no reason. Pure; it judges
// the node's taints and labels and nothing else (no cordon, readiness or
// capacity).
func placementOn(pods []corev1.Pod, node corev1.Node) nodePlacement {
	var p nodePlacement
	seenTaints := map[findings.NodeTaint]bool{}
	var labels []findings.LabelMismatch
	for _, pod := range pods {
		untolerated := untoleratedTaints(pod, node)
		unmatched := unmatchedNodeLabels(pod, node)
		p.TaintRoom = p.TaintRoom || len(untolerated) == 0
		p.LabelRoom = p.LabelRoom || len(unmatched) == 0
		p.Room = p.Room || (len(untolerated) == 0 && len(unmatched) == 0)
		for _, t := range untolerated {
			seenTaints[findings.NodeTaint{Key: t.Key, Effect: string(t.Effect)}] = true
		}
		for _, m := range unmatched {
			if !slices.ContainsFunc(labels, func(o findings.LabelMismatch) bool { return reflect.DeepEqual(o, m) }) {
				labels = append(labels, m)
			}
		}
	}
	if p.Room {
		return p
	}
	for _, t := range node.Spec.Taints {
		nt := findings.NodeTaint{Key: t.Key, Effect: string(t.Effect)}
		if seenTaints[nt] {
			p.Reasons = append(p.Reasons, placementReason{Kind: placementReasonTaint, Taint: nt})
			delete(seenTaints, nt)
		}
	}
	slices.SortFunc(labels, compareLabelMismatch)
	for _, m := range labels {
		p.Reasons = append(p.Reasons, placementReason{Kind: placementReasonLabel, Label: m})
	}
	return p
}

// compareLabelMismatch orders requirements by source, key, operator and values,
// so that evidence does not depend on the order of the pods.
func compareLabelMismatch(a, b findings.LabelMismatch) int {
	return cmp.Or(
		cmp.Compare(a.Source, b.Source), cmp.Compare(a.Key, b.Key), cmp.Compare(a.Operator, b.Operator),
		slices.Compare(a.Values, b.Values), cmp.Compare(a.NodeValue, b.NodeValue))
}

// taints returns the reasons that are taints, in order, for the evidence of the
// taint rules.
func (p nodePlacement) taints() []findings.NodeTaint {
	var out []findings.NodeTaint
	for _, r := range p.Reasons {
		if r.Kind == placementReasonTaint {
			out = append(out, r.Taint)
		}
	}
	return out
}

// labels returns the reasons that are label requirements, in order, for the
// evidence of the label rule.
func (p nodePlacement) labels() []findings.LabelMismatch {
	var out []findings.LabelMismatch
	for _, r := range p.Reasons {
		if r.Kind == placementReasonLabel {
			out = append(out, r.Label)
		}
	}
	return out
}
