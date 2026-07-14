package api

import (
	"os"
	"testing"
)

// Temporary profiling harness: run with
// CSDA_PROFILE_DEMO=<path> go test ./pkg/api -run TestProfileDemoAnalysis -cpuprofile cpu.out
func TestProfileDemoAnalysis(t *testing.T) {
	path := os.Getenv("CSDA_PROFILE_DEMO")
	if path == "" {
		t.Skip("CSDA_PROFILE_DEMO not set")
	}
	result := analyzeOneDemoForStats(path, PlayerStatsBuildOptions{Source: "valve", TrisDir: "../../tris"})
	if result.err != nil {
		t.Fatal(result.err)
	}
	t.Logf("encounters: %d", len(result.stats.Encounters))
}
