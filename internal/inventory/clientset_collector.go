package inventory

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ClientsetCollector is a Collector backed by a client-go clientset.
// It only issues list calls; it never writes to the cluster.
type ClientsetCollector struct {
	client kubernetes.Interface
	logger *slog.Logger
	now    func() time.Time
}

// NewClientsetCollector returns a collector that reads through client. It
// writes one DEBUG line per API call to logger; a nil logger discards them.
func NewClientsetCollector(client kubernetes.Interface, logger *slog.Logger) *ClientsetCollector {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &ClientsetCollector{client: client, logger: logger, now: time.Now}
}

// Collect lists nodes, pods, ReplicaSets, PodDisruptionBudgets, Deployments,
// StatefulSets, Jobs and CronJobs (all namespaces) and returns them as a
// Snapshot.
func (c *ClientsetCollector) Collect(ctx context.Context) (*Snapshot, error) {
	taken := c.now()

	start := time.Now()
	nodes, err := c.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, c.listFailed("nodes", start, err)
	}
	c.listed("nodes", start, len(nodes.Items))

	start = time.Now()
	pods, err := c.client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, c.listFailed("pods", start, err)
	}
	c.listed("pods", start, len(pods.Items))

	start = time.Now()
	rss, err := c.client.AppsV1().ReplicaSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, c.listFailed("replicasets", start, err)
	}
	c.listed("replicasets", start, len(rss.Items))

	start = time.Now()
	pdbs, err := c.client.PolicyV1().PodDisruptionBudgets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, c.listFailed("poddisruptionbudgets", start, err)
	}
	c.listed("poddisruptionbudgets", start, len(pdbs.Items))

	workloads, err := c.listWorkloads(ctx)
	if err != nil {
		return nil, err
	}

	return NewSnapshot(nodes.Items, pods.Items, rss.Items, pdbs.Items, taken, WithWorkloads(workloads)), nil
}

// listed logs a successful list call of resource.
func (c *ClientsetCollector) listed(resource string, start time.Time, count int) {
	c.logger.Debug("list", "resource", resource, "count", count, "duration", time.Since(start))
}

// listFailed logs a failed list call of resource and returns the error Collect
// reports for it.
func (c *ClientsetCollector) listFailed(resource string, start time.Time, err error) error {
	c.logger.Debug("list failed", "resource", resource, "duration", time.Since(start), "error", err)
	return fmt.Errorf("list %s: %w", resource, err)
}
