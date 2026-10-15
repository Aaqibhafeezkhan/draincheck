package findings

import (
	"fmt"
	"reflect"
	"testing"
)

func ExampleSort() {
	fs := []Finding{
		{Resource: ResourceRef{Namespace: "web", Kind: "Pod", Name: "zeta"}},
		{Resource: ResourceRef{Namespace: "web", Kind: "Pod", Name: "api"}},
		{Resource: ResourceRef{Namespace: "web", Kind: "Deployment", Name: "frontend"}},
		{Resource: ResourceRef{Namespace: "billing", Kind: "Pod", Name: "worker"}},
	}

	Sort(fs)
	for _, f := range fs {
		fmt.Println(f.Resource)
	}

	// Output:
	// Pod/billing/worker
	// Deployment/web/frontend
	// Pod/web/api
	// Pod/web/zeta
}

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

func TestSortTieBreaks(t *testing.T) {
	r := ResourceRef{Namespace: "web", Kind: "Pod", Name: "p"}
	in := []Finding{
		{RuleID: "B", Severity: Risk, Resource: r},
		{RuleID: "A", Severity: Unknown, Resource: r},
		{RuleID: "A", Severity: Blocked, Resource: r},
	}
	Sort(in)

	got := []string{in[0].RuleID + "/" + string(in[0].Severity), in[1].RuleID + "/" + string(in[1].Severity), in[2].RuleID + "/" + string(in[2].Severity)}
	want := []string{"A/BLOCKED", "A/UNKNOWN", "B/RISK"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSortEmpty(t *testing.T) {
	Sort(nil)
	Sort([]Finding{})
}
