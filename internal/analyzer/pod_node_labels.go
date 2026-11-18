package analyzer

import (
	"slices"
	"strconv"

	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
)

// unmatchedNodeLabels returns the requirements of pod that node does not meet:
// every nodeSelector entry the node's labels lack, and, when no term of the
// required node affinity matches, the first unmet requirement of each term. An
// empty result means the node's labels leave room for the pod.
//
// Only required constraints rule a node out; a preferred term never does.
// Anything draincheck cannot evaluate counts as open, so nothing is reported on
// a guess: an unknown operator, In or NotIn without values, Gt or Lt without
// exactly one integer, a matchFields key other than metadata.name, a term with
// no requirement, and a required affinity with no term. Matching is the
// scheduler's own (In, NotIn, Exists, DoesNotExist, Gt and Lt; NotIn and
// DoesNotExist match a node without the key; Gt and Lt compare integers).
func unmatchedNodeLabels(pod corev1.Pod, node corev1.Node) []findings.LabelMismatch {
	var out []findings.LabelMismatch

	keys := make([]string, 0, len(pod.Spec.NodeSelector))
	for k := range pod.Spec.NodeSelector {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		want := pod.Spec.NodeSelector[k]
		if have, ok := node.Labels[k]; !ok || have != want {
			out = append(out, labelMismatch(findings.LabelSourceNodeSelector, k, "In", []string{want}, node.Labels[k], ok))
		}
	}

	return append(out, unmatchedRequiredAffinity(pod, node)...)
}

// unmatchedRequiredAffinity is the node affinity half of unmatchedNodeLabels.
func unmatchedRequiredAffinity(pod corev1.Pod, node corev1.Node) []findings.LabelMismatch {
	aff := pod.Spec.Affinity
	if aff == nil || aff.NodeAffinity == nil || aff.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return nil
	}
	terms := aff.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	if len(terms) == 0 {
		return nil
	}
	var out []findings.LabelMismatch
	for _, term := range terms {
		mismatch, open := unmetInTerm(term, node)
		if open {
			return nil
		}
		if mismatch == nil {
			return nil // this term matches: the node is room
		}
		out = append(out, *mismatch)
	}
	return out
}

// unmetInTerm judges one term, whose requirements are ANDed. It returns the
// first requirement that is definitely unmet, or nil with open=false when all
// are met. open is true when no requirement is definitely unmet but one cannot
// be evaluated, or the term has none.
func unmetInTerm(term corev1.NodeSelectorTerm, node corev1.Node) (mismatch *findings.LabelMismatch, open bool) {
	if len(term.MatchExpressions) == 0 && len(term.MatchFields) == 0 {
		return nil, true
	}
	unsupported := false
	check := func(source string, req corev1.NodeSelectorRequirement, value string, present bool) {
		if mismatch != nil {
			return
		}
		switch judgeRequirement(req, value, present) {
		case requirementUnmet:
			m := labelMismatch(source, req.Key, string(req.Operator), req.Values, value, present)
			mismatch = &m
		case requirementUnsupported:
			unsupported = true
		}
	}
	for _, req := range term.MatchExpressions {
		value, present := node.Labels[req.Key]
		check(findings.LabelSourceNodeAffinity, req, value, present)
	}
	for _, req := range term.MatchFields {
		if req.Key != "metadata.name" {
			unsupported = true
			continue
		}
		check(findings.LabelSourceNodeAffinityField, req, node.Name, true)
	}
	if mismatch != nil {
		return mismatch, false
	}
	return nil, unsupported
}

type requirementResult int

const (
	requirementMet requirementResult = iota
	requirementUnmet
	requirementUnsupported
)

// judgeRequirement judges one requirement against the value a node has for its key.
func judgeRequirement(req corev1.NodeSelectorRequirement, value string, present bool) requirementResult {
	met := func(ok bool) requirementResult {
		if ok {
			return requirementMet
		}
		return requirementUnmet
	}
	switch req.Operator {
	case corev1.NodeSelectorOpIn:
		if len(req.Values) == 0 {
			return requirementUnsupported
		}
		return met(present && slices.Contains(req.Values, value))
	case corev1.NodeSelectorOpNotIn:
		if len(req.Values) == 0 {
			return requirementUnsupported
		}
		return met(!present || !slices.Contains(req.Values, value))
	case corev1.NodeSelectorOpExists:
		return met(present)
	case corev1.NodeSelectorOpDoesNotExist:
		return met(!present)
	case corev1.NodeSelectorOpGt, corev1.NodeSelectorOpLt:
		if len(req.Values) != 1 {
			return requirementUnsupported
		}
		want, err := strconv.ParseInt(req.Values[0], 10, 64)
		if err != nil {
			return requirementUnsupported
		}
		have, err := strconv.ParseInt(value, 10, 64)
		if !present || err != nil {
			return requirementUnmet
		}
		if req.Operator == corev1.NodeSelectorOpGt {
			return met(have > want)
		}
		return met(have < want)
	}
	return requirementUnsupported
}

// labelMismatch builds the evidence of one unmet requirement. Values is never
// nil, so the JSON has an empty list, not null.
func labelMismatch(source, key, operator string, values []string, nodeValue string, nodeHasKey bool) findings.LabelMismatch {
	return findings.LabelMismatch{
		Source: source, Key: key, Operator: operator,
		Values:     append([]string{}, values...),
		NodeHasKey: nodeHasKey, NodeValue: nodeValue,
	}
}
