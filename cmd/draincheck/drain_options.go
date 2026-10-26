package main

import (
	"fmt"

	"github.com/Veinar/draincheck/internal/analyzer"
	"k8s.io/apimachinery/pkg/labels"
)

// drainOptions turns the drain flags into what the analyzer reads. It fails on a
// --pod-selector that is not a label selector.
func (o options) drainOptions() (analyzer.DrainOptions, error) {
	d := analyzer.DrainOptions{
		IgnoreDaemonSets:   o.IgnoreDaemonSets,
		DeleteEmptyDirData: o.DeleteEmptyDirData,
		Force:              o.Force,
		DisableEviction:    o.DisableEviction,
	}
	if o.PodSelector != "" {
		sel, err := labels.Parse(o.PodSelector)
		if err != nil {
			return d, fmt.Errorf("invalid --pod-selector %q: %w", o.PodSelector, err)
		}
		d.PodSelector = sel
	}
	return d, nil
}
