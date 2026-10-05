package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeBrokenDemo(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBrokenDemosFailTheirOwnAnalysisOnly(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeBrokenDemo(t, dir, "empty.dem", nil),
		writeBrokenDemo(t, dir, "garbage.dem", []byte("this is definitely not a demo file, just some text")),
		writeBrokenDemo(t, dir, "cut.dem", append([]byte("HL2DEMO\x00"), make([]byte, 100)...)),
	}
	result, err := BuildPlayerStatsDatabase(context.Background(), PlayerStatsBuildOptions{
		DatabasePath: filepath.Join(dir, "stats.db"), DemoPaths: paths, Jobs: 3, TrisDir: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed != len(paths) || result.Imported != 0 || len(result.Errors) != len(paths) {
		t.Fatalf("result=%+v, want every broken demo reported as failed", result)
	}
	for _, item := range result.Errors {
		if item.Error == "" {
			t.Fatalf("empty error for %s", item.Path)
		}
	}
}

func TestAnalyzeDemoReturnsErrorForEmptyFile(t *testing.T) {
	path := writeBrokenDemo(t, t.TempDir(), "empty.dem", nil)
	if _, err := AnalyzeDemo(path, AnalyzeDemoOptions{}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestCancelledBuildAnalyzesNothing(t *testing.T) {
	dir := t.TempDir()
	path := writeBrokenDemo(t, dir, "empty.dem", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := BuildPlayerStatsDatabase(ctx, PlayerStatsBuildOptions{DatabasePath: filepath.Join(dir, "stats.db"), DemoPaths: []string{path}, TrisDir: dir})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
	if result.Failed != 0 || result.Imported != 0 {
		t.Fatalf("result=%+v, an interrupted build must not report failures", result)
	}
}
