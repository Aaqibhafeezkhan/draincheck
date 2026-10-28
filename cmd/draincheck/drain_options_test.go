package main

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"
)

// FuzzDrainOptionsPodSelector feeds --pod-selector arbitrary text. It must never
// panic, and a selector it accepts must survive printing and parsing again,
// because the debug output and the error messages print it.
func FuzzDrainOptionsPodSelector(f *testing.F) {
	f.Add("")
	f.Add("app=web")
	f.Add("tier in (a,b),!canary")
	f.Add("a=b,")
	f.Add("=")
	f.Add("a in ()")
	f.Fuzz(func(t *testing.T, selector string) {
		d, err := options{PodSelector: selector}.drainOptions()
		if err != nil || d.PodSelector == nil {
			return
		}
		again, err := labels.Parse(d.PodSelector.String())
		if err != nil {
			t.Fatalf("selector %q prints as %q, which does not parse: %v", selector, d.PodSelector, err)
		}
		if again.String() != d.PodSelector.String() {
			t.Fatalf("selector %q: %q != %q after a round trip", selector, again, d.PodSelector)
		}
		d.PodSelector.Matches(labels.Set{"app": "web"})
	})
}
