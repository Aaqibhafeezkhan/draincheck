package inventory

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"
)

// permission is one verb on one resource of one API group, the unit both the
// collector and the role are reduced to.
type permission struct {
	Group, Resource, Verb string
}

func (p permission) String() string {
	if p.Group == "" {
		return p.Verb + " " + p.Resource
	}
	return p.Verb + " " + p.Resource + "." + p.Group
}

const (
	exampleRolePath    = "../../examples/rbac/clusterrole.yaml"
	exampleBindingPath = "../../examples/rbac/clusterrolebinding.yaml"
)

// TestExampleClusterRoleMatchesCollector guards the shipped role in both
// directions: every call the collector makes is granted (a new List without a
// rule makes the role insufficient), and every grant is used (no leftover
// permission). It needs no cluster.
func TestExampleClusterRoleMatchesCollector(t *testing.T) {
	needed := collectorPermissions(t)
	granted := grantedPermissions(t, loadExampleRole(t))

	missing, unused := rbacDrift(needed, granted)
	for _, p := range missing {
		t.Errorf("the collector needs %q but examples/rbac/clusterrole.yaml does not grant it", p)
	}
	for _, p := range unused {
		t.Errorf("examples/rbac/clusterrole.yaml grants %q but the collector never uses it", p)
	}
}

// TestExampleBindingPointsAtExampleRole catches a renamed role: the binding
// would apply cleanly and grant nothing.
func TestExampleBindingPointsAtExampleRole(t *testing.T) {
	role := loadExampleRole(t)

	raw, err := os.ReadFile(exampleBindingPath)
	if err != nil {
		t.Fatal(err)
	}
	var binding rbacv1.ClusterRoleBinding
	if err := yaml.UnmarshalStrict(raw, &binding); err != nil {
		t.Fatalf("parse %s: %v", exampleBindingPath, err)
	}
	if binding.Kind != "ClusterRoleBinding" {
		t.Errorf("kind = %q, want ClusterRoleBinding", binding.Kind)
	}
	if binding.RoleRef.Kind != "ClusterRole" || binding.RoleRef.Name != role.Name {
		t.Errorf("roleRef = %s/%s, want ClusterRole/%s", binding.RoleRef.Kind, binding.RoleRef.Name, role.Name)
	}
	if len(binding.Subjects) == 0 {
		t.Error("the binding has no subject")
	}
}

func TestRBACDrift(t *testing.T) {
	list := func(group, resource string) permission {
		return permission{Group: group, Resource: resource, Verb: "list"}
	}
	tests := []struct {
		name        string
		needed      []permission
		granted     []permission
		wantMissing []permission
		wantUnused  []permission
	}{
		{
			name:    "exact match",
			needed:  []permission{list("", "pods"), list("apps", "deployments")},
			granted: []permission{list("apps", "deployments"), list("", "pods")},
		},
		{
			name:        "resource the role lacks is named",
			needed:      []permission{list("", "pods"), list("apps", "daemonsets")},
			granted:     []permission{list("", "pods")},
			wantMissing: []permission{list("apps", "daemonsets")},
		},
		{
			name:       "resource nothing uses is named",
			needed:     []permission{list("", "pods")},
			granted:    []permission{list("", "pods"), list("", "secrets")},
			wantUnused: []permission{list("", "secrets")},
		},
		{
			name:        "same resource in the wrong group does not count",
			needed:      []permission{list("apps", "deployments")},
			granted:     []permission{list("extensions", "deployments")},
			wantMissing: []permission{list("apps", "deployments")},
			wantUnused:  []permission{list("extensions", "deployments")},
		},
		{
			name:        "another verb does not cover list",
			needed:      []permission{list("", "pods")},
			granted:     []permission{{Resource: "pods", Verb: "get"}},
			wantMissing: []permission{list("", "pods")},
			wantUnused:  []permission{{Resource: "pods", Verb: "get"}},
		},
		{
			name:        "a wildcard is never enough and never accepted",
			needed:      []permission{list("", "pods")},
			granted:     []permission{{Group: "*", Resource: "*", Verb: "*"}},
			wantMissing: []permission{list("", "pods")},
			wantUnused:  []permission{{Group: "*", Resource: "*", Verb: "*"}},
		},
		{
			name:    "a duplicate grant is not drift",
			needed:  []permission{list("", "pods")},
			granted: []permission{list("", "pods"), list("", "pods")},
		},
		{
			name:    "a duplicate need is reported once",
			needed:  []permission{list("", "pods"), list("", "pods")},
			granted: []permission{list("", "pods")},
		},
		{
			name:        "nothing granted",
			needed:      []permission{list("", "nodes")},
			wantMissing: []permission{list("", "nodes")},
		},
		{
			name: "nothing needed, nothing granted",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing, unused := rbacDrift(tt.needed, tt.granted)
			if !reflect.DeepEqual(missing, tt.wantMissing) {
				t.Errorf("missing = %v, want %v", missing, tt.wantMissing)
			}
			if !reflect.DeepEqual(unused, tt.wantUnused) {
				t.Errorf("unused = %v, want %v", unused, tt.wantUnused)
			}
		})
	}
}

func TestGrantedPermissionsExpandsRules(t *testing.T) {
	role := &rbacv1.ClusterRole{Rules: []rbacv1.PolicyRule{
		{APIGroups: []string{"", "apps"}, Resources: []string{"pods", "deployments"}, Verbs: []string{"list", "watch"}},
	}}
	got := grantedPermissions(t, role)
	if len(got) != 8 {
		t.Fatalf("got %d permissions %v, want 2 groups x 2 resources x 2 verbs = 8", len(got), got)
	}
}

func TestGrantedPermissionsRejectsUnmodelledRuleFields(t *testing.T) {
	// resourceNames, nonResourceURLs and aggregation would let the role differ
	// from what the comparison sees, so the helper must refuse them rather
	// than ignore them.
	for name, rule := range map[string]rbacv1.PolicyRule{
		"resourceNames":   {APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"list"}, ResourceNames: []string{"x"}},
		"nonResourceURLs": {NonResourceURLs: []string{"/healthz"}, Verbs: []string{"get"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := expandRule(rule); err == nil {
				t.Error("expandRule() error = nil, want an error")
			}
		})
	}
}

func TestCollectorPermissionsAreLists(t *testing.T) {
	// A collector call that is not a list would be a write or a watch; the
	// permission set has to show it so the comparison above fails on it.
	for _, p := range collectorPermissions(t) {
		if p.Verb != "list" {
			t.Errorf("collector makes %q, want list only", p)
		}
	}
}

func collectorPermissions(t *testing.T) []permission {
	t.Helper()
	client := fake.NewClientset()
	if _, err := NewClientsetCollector(client, nil).Collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	var out []permission
	for _, a := range client.Actions() {
		gvr := a.GetResource()
		out = append(out, permission{Group: gvr.Group, Resource: gvr.Resource, Verb: a.GetVerb()})
	}
	if len(out) == 0 {
		t.Fatal("the collector made no API call; the comparison would pass for nothing")
	}
	return out
}

func loadExampleRole(t *testing.T) *rbacv1.ClusterRole {
	t.Helper()
	raw, err := os.ReadFile(exampleRolePath)
	if err != nil {
		t.Fatal(err)
	}
	var role rbacv1.ClusterRole
	if err := yaml.UnmarshalStrict(raw, &role); err != nil {
		t.Fatalf("parse %s: %v", exampleRolePath, err)
	}
	if role.Kind != "ClusterRole" {
		t.Fatalf("kind = %q, want ClusterRole", role.Kind)
	}
	return &role
}

func grantedPermissions(t *testing.T, role *rbacv1.ClusterRole) []permission {
	t.Helper()
	var out []permission
	for _, rule := range role.Rules {
		ps, err := expandRule(rule)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, ps...)
	}
	return out
}

// expandRule turns a rule into one permission per group, resource and verb.
// A rule that narrows by name or grants a URL cannot be compared this way, so
// it is an error instead of being ignored.
func expandRule(rule rbacv1.PolicyRule) ([]permission, error) {
	if len(rule.ResourceNames) > 0 || len(rule.NonResourceURLs) > 0 {
		return nil, fmt.Errorf("rule %+v uses resourceNames or nonResourceURLs, which the comparison does not model", rule)
	}
	var out []permission
	for _, g := range rule.APIGroups {
		for _, r := range rule.Resources {
			for _, v := range rule.Verbs {
				out = append(out, permission{Group: g, Resource: r, Verb: v})
			}
		}
	}
	return out, nil
}

// rbacDrift returns what is needed but not granted, and what is granted but
// not needed. Both are sorted and free of duplicates.
func rbacDrift(needed, granted []permission) (missing, unused []permission) {
	return subtract(needed, granted), subtract(granted, needed)
}

func subtract(a, b []permission) []permission {
	in := make(map[permission]bool, len(b))
	for _, p := range b {
		in[p] = true
	}
	var out []permission
	seen := map[permission]bool{}
	for _, p := range a {
		if in[p] || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}
