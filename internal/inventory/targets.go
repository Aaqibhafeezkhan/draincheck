package inventory

import (
	"errors"
	"slices"
	"strings"
)

// NodesNotFoundError lists requested nodes that do not exist in the cluster.
type NodesNotFoundError struct {
	Names []string
}

func (e *NodesNotFoundError) Error() string {
	return "node(s) not found in cluster: " + strings.Join(e.Names, ", ")
}

// ResolveTargetNodes returns the sorted, de-duplicated names of the nodes to
// analyze. With all set, every node in the snapshot is selected. Otherwise
// each requested name must exist in the snapshot, or a *NodesNotFoundError
// listing the missing names is returned.
func ResolveTargetNodes(s *Snapshot, requested []string, all bool) ([]string, error) {
	known := make(map[string]struct{}, len(s.nodes))
	for i := range s.nodes {
		known[s.nodes[i].Name] = struct{}{}
	}

	if all {
		if len(known) == 0 {
			return nil, errors.New("cluster has no nodes")
		}
		names := make([]string, 0, len(known))
		for n := range known {
			names = append(names, n)
		}
		slices.Sort(names)
		return names, nil
	}

	var targets, missing []string
	for _, n := range requested {
		if _, ok := known[n]; ok {
			targets = append(targets, n)
		} else {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, &NodesNotFoundError{Names: slices.Compact(missing)}
	}
	if len(targets) == 0 {
		return nil, errors.New("no target nodes given")
	}
	slices.Sort(targets)
	return slices.Compact(targets), nil
}
