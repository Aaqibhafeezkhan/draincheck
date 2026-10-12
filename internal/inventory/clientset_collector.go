package inventory

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ClientsetCollector is a Collector backed by a client-go clientset.
// It only issues list calls; it never writes to the cluster.
type ClientsetCollector struct {
	client kubernetes.Interface
	now    func() time.Time
}

// NewClientsetCollector returns a collector that reads through client.
func NewClientsetCollector(client kubernetes.Interface) *ClientsetCollector {
	return &ClientsetCollector{client: client, now: time.Now}
}

// Collect lists nodes, pods (all namespaces) and PodDisruptionBudgets
// (all namespaces) and returns them as a Snapshot.
func (c *ClientsetCollector) Collect(ctx context.Context) (*Snapshot, error) {
	taken := c.now()

	nodes, err := c.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	pods, err := c.client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	pdbs, err := c.client.PolicyV1().PodDisruptionBudgets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list poddisruptionbudgets: %w", err)
	}

	return NewSnapshot(nodes.Items, pods.Items, pdbs.Items, taken), nil
}
