package findings

import (
	"reflect"
	"testing"
)

func TestSort(t *testing.T) {
	ref := func(ns, kind, name string) ResourceRef {
		return ResourceRef{Namespace: ns, Kind: kind, Name: name}
	}
	in := []Finding{
		{RuleID: "B", Resource: ref("web", "Deployment", "b")},
		{RuleID: "A", Resource: ref("web", "Deployment", "b")},
		{RuleID: "A", Resource: ref("web", "Deployment", "a")},
		{RuleID: "A", Resource: ref("web", "DaemonSet", "z")},
		{RuleID: "A", Resource: ref("billing", "Pod", "p")},
		{RuleID: "A", Resource: ref("", "Node", "worker-01")},
	}
	want := []Finding{
		{RuleID: "A", Resource: ref("", "Node", "worker-01")},
		{RuleID: "A", Resource: ref("billing", "Pod", "p")},
		{RuleID: "A", Resource: ref("web", "DaemonSet", "z")},
		{RuleID: "A", Resource: ref("web", "Deployment", "a")},
		{RuleID: "A", Resource: ref("web", "Deployment", "b")},
		{RuleID: "B", Resource: ref("web", "Deployment", "b")},
	}

	Sort(in)
	if !reflect.DeepEqual(in, want) {
		t.Errorf("Sort() order mismatch\n got: %+v\nwant: %+v", in, want)
	}

	// Sorting must be independent of the input order.
	rev := make([]Finding, len(want))
	for i, f := range want {
		rev[len(want)-1-i] = f
	}
	Sort(rev)
	if !reflect.DeepEqual(rev, want) {
		t.Errorf("Sort() not deterministic for reversed input\n got: %+v", rev)
	}
}
