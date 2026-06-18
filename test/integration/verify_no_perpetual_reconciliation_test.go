package integration

import (
	"fmt"
	"io"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// settlingPeriod is the time to wait after the synthetic update is reconciled
// to confirm no further updates occur.
const settlingPeriod = 30 * time.Second

// verifyNoDeploymentUpdates asserts that the reconcile counter recorded exactly
// the expected number of deployment updates, then resets the counter so the
// next measurement window starts from zero.
//
// Before the synthetic trigger this should be 0 (no perpetual reconcile during
// initial deployment). After the synthetic trigger it should be exactly 1 (the
// operator corrected the drift and stopped).
func verifyNoDeploymentUpdates(expected int, w io.Writer) {
	updateCounts := reconcileCounter.GetUpdateCounts()
	totalUpdates := reconcileCounter.GetTotalUpdates()

	fmt.Fprintf(w, "\n=== Deployment Update Report (expected %d) ===\n", expected)
	fmt.Fprintf(w, "Total: %d\n", totalUpdates)

	names := make([]string, 0, len(updateCounts))
	for n := range updateCounts {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(w, "  %s: %d\n", n, updateCounts[n])
	}
	fmt.Fprintf(w, "==============================================\n\n")

	Expect(totalUpdates).To(Equal(expected),
		deploymentUpdateDetail(updateCounts, totalUpdates, expected))

	reconcileCounter.Reset()
}

// deploymentUpdateDetail builds a human-readable breakdown for use in a Gomega
// failure message so the offending deployments are immediately visible.
func deploymentUpdateDetail(counts map[string]int, total, expected int) string {
	sb := fmt.Sprintf("total deployment updates %d, expected %d; per-deployment breakdown:\n",
		total, expected)
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		sb += fmt.Sprintf("  %s: %d\n", n, counts[n])
	}
	return sb
}

var _ = Describe // suppress unused import lint for ginkgo dot-import
