package analyzer

import "slices"

// CountSelectedPods counts the pods on the target nodes that --pod-selector
// keeps in the drain and the ones it leaves out. Without a selector every pod
// is selected.
func CountSelectedPods(in Input) (selected, leftOut int) {
	for _, pod := range in.Snapshot.Pods() {
		if !slices.Contains(in.Targets, pod.Spec.NodeName) {
			continue
		}
		if in.Drain.selects(pod) {
			selected++
		} else {
			leftOut++
		}
	}
	return selected, leftOut
}
