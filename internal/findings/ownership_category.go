package findings

// OwnershipCategory classifies why a pod has special drain behaviour.
type OwnershipCategory string

const (
	// OwnershipDaemonSet is a pod managed by a DaemonSet. kubectl drain
	// refuses it unless --ignore-daemonsets is set, and never evicts it.
	OwnershipDaemonSet OwnershipCategory = "DaemonSet"
	// OwnershipStaticPod is the API mirror of a static pod run by the kubelet.
	// It cannot be evicted or rescheduled; it stops with its node.
	OwnershipStaticPod OwnershipCategory = "StaticPod"
	// OwnershipUnmanaged is a pod with no controller. kubectl drain refuses it
	// unless --force is set, and nothing recreates it once deleted.
	OwnershipUnmanaged OwnershipCategory = "Unmanaged"
)
