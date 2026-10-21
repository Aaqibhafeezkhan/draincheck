package inventory

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
)

// Workloads are the controllers that own pods and the Jobs they create.
type Workloads struct {
	Deployments  []appsv1.Deployment
	StatefulSets []appsv1.StatefulSet
	Jobs         []batchv1.Job
	CronJobs     []batchv1.CronJob
}

// WithWorkloads adds the workload objects to a Snapshot. Without it the
// snapshot holds none.
func WithWorkloads(w Workloads) SnapshotOption {
	return func(s *Snapshot) {
		s.deployments = copyDeployments(w.Deployments)
		s.statefulSets = copyStatefulSets(w.StatefulSets)
		s.jobs = copyJobs(w.Jobs)
		s.cronJobs = copyCronJobs(w.CronJobs)
	}
}

// Deployments returns a copy of the Deployments in the snapshot.
func (s *Snapshot) Deployments() []appsv1.Deployment { return copyDeployments(s.deployments) }

// StatefulSets returns a copy of the StatefulSets in the snapshot.
func (s *Snapshot) StatefulSets() []appsv1.StatefulSet { return copyStatefulSets(s.statefulSets) }

// Jobs returns a copy of the Jobs in the snapshot.
func (s *Snapshot) Jobs() []batchv1.Job { return copyJobs(s.jobs) }

// CronJobs returns a copy of the CronJobs in the snapshot.
func (s *Snapshot) CronJobs() []batchv1.CronJob { return copyCronJobs(s.cronJobs) }

func copyDeployments(in []appsv1.Deployment) []appsv1.Deployment {
	out := make([]appsv1.Deployment, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}

func copyStatefulSets(in []appsv1.StatefulSet) []appsv1.StatefulSet {
	out := make([]appsv1.StatefulSet, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}

func copyJobs(in []batchv1.Job) []batchv1.Job {
	out := make([]batchv1.Job, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}

func copyCronJobs(in []batchv1.CronJob) []batchv1.CronJob {
	out := make([]batchv1.CronJob, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}
