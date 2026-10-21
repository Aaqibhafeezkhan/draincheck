package inventory

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// listStorage lists PersistentVolumeClaims (all namespaces) and
// PersistentVolumes. A failed list is an error, never an empty result: an
// empty list would hide findings.
func (c *ClientsetCollector) listStorage(ctx context.Context) (Storage, error) {
	var st Storage

	start := time.Now()
	pvcs, err := c.client.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return Storage{}, c.listFailed("persistentvolumeclaims", start, err)
	}
	c.listed("persistentvolumeclaims", start, len(pvcs.Items))
	st.PersistentVolumeClaims = pvcs.Items

	start = time.Now()
	pvs, err := c.client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return Storage{}, c.listFailed("persistentvolumes", start, err)
	}
	c.listed("persistentvolumes", start, len(pvs.Items))
	st.PersistentVolumes = pvs.Items

	return st, nil
}
