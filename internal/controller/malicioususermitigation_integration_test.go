package controller

import (
	"testing"
	"time"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func waitForMaliciousUserMitigationConditionResult(t *testing.T, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus, timeout time.Duration) *v1alpha1.MaliciousUserMitigation {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var cr v1alpha1.MaliciousUserMitigation
		if err := testClient.Get(testCtx, key, &cr); err == nil {
			if c := meta.FindStatusCondition(cr.Status.Conditions, condType); c != nil && c.Status == wantStatus {
				return &cr
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s=%s on %s", condType, wantStatus, key)
	return nil
}
