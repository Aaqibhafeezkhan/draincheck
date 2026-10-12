package inventory

import "context"

// Collector gathers a read-only Snapshot of the cluster.
type Collector interface {
	Collect(ctx context.Context) (*Snapshot, error)
}
