package inventory

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// listWorkloads lists Deployments, StatefulSets, Jobs and CronJobs (all
// namespaces). A failed list is an error, never an empty result: an empty list
// would hide findings.
func (c *ClientsetCollector) listWorkloads(ctx context.Context) (Workloads, error) {
	var w Workloads

	start := time.Now()
	deployments, err := c.client.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return Workloads{}, c.listFailed("deployments", start, err)
	}
	c.listed("deployments", start, len(deployments.Items))
	w.Deployments = deployments.Items

	start = time.Now()
	statefulSets, err := c.client.AppsV1().StatefulSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return Workloads{}, c.listFailed("statefulsets", start, err)
	}
	c.listed("statefulsets", start, len(statefulSets.Items))
	w.StatefulSets = statefulSets.Items

	start = time.Now()
	jobs, err := c.client.BatchV1().Jobs(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return Workloads{}, c.listFailed("jobs", start, err)
	}
	c.listed("jobs", start, len(jobs.Items))
	w.Jobs = jobs.Items

	start = time.Now()
	cronJobs, err := c.client.BatchV1().CronJobs(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return Workloads{}, c.listFailed("cronjobs", start, err)
	}
	c.listed("cronjobs", start, len(cronJobs.Items))
	w.CronJobs = cronJobs.Items

	return w, nil
}
