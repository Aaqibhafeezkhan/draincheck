package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of PodLocalStorage: BLOCKED without --delete-emptydir-data, RISK
// with it.
const (
	podLocalStorageRefused = "emptydir_refused"
	podLocalStorageDeleted = "emptydir_deleted"
)

const podLocalStorageConfirm = "Confirm with `kubectl get pod -n <namespace> <pod> -o jsonpath='{.spec.volumes}'` (an entry with emptyDir is local storage). "

// podLocalStorageCases holds the text of each case, keyed as findings.CaseKey
// does.
var podLocalStorageCases = findings.Catalog{
	findings.CaseKey(PodLocalStorageID, podLocalStorageRefused): {
		Why:         "kubectl drain refuses these pods without --delete-emptydir-data (cannot delete Pods with local storage) and stops before it evicts anything; waiting does not change that.",
		Remediation: podLocalStorageConfirm + "Then, safest first: move data that must survive to a PersistentVolumeClaim; or, if the volumes only hold a cache or scratch data, drain with --delete-emptydir-data and accept that the data is lost.",
	},
	findings.CaseKey(PodLocalStorageID, podLocalStorageDeleted): {
		Why:         "With --delete-emptydir-data kubectl drain deletes the pods and the data in those volumes is lost; a controller recreates the pod with an empty volume.",
		Remediation: podLocalStorageConfirm + "Then, safest first: move data that must survive to a PersistentVolumeClaim before the drain; or accept the loss, which is what --delete-emptydir-data already says.",
	},
}
