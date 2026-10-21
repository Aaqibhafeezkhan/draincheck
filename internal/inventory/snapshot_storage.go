package inventory

import corev1 "k8s.io/api/core/v1"

// Storage are the volume objects a pod can depend on.
type Storage struct {
	PersistentVolumeClaims []corev1.PersistentVolumeClaim
	PersistentVolumes      []corev1.PersistentVolume
}

// WithStorage adds the volume objects to a Snapshot. Without it the snapshot
// holds none.
func WithStorage(st Storage) SnapshotOption {
	return func(s *Snapshot) {
		s.pvcs = copyPVCs(st.PersistentVolumeClaims)
		s.pvs = copyPVs(st.PersistentVolumes)
	}
}

// PersistentVolumeClaims returns a copy of the PersistentVolumeClaims in the snapshot.
func (s *Snapshot) PersistentVolumeClaims() []corev1.PersistentVolumeClaim { return copyPVCs(s.pvcs) }

// PersistentVolumes returns a copy of the PersistentVolumes in the snapshot.
func (s *Snapshot) PersistentVolumes() []corev1.PersistentVolume { return copyPVs(s.pvs) }

func copyPVCs(in []corev1.PersistentVolumeClaim) []corev1.PersistentVolumeClaim {
	out := make([]corev1.PersistentVolumeClaim, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}

func copyPVs(in []corev1.PersistentVolume) []corev1.PersistentVolume {
	out := make([]corev1.PersistentVolume, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}
