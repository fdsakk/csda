package demo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGetDemoFromPathRejectsBrokenFilesWithoutPanicking(t *testing.T) {
	cases := map[string][]byte{
		"empty":                nil,
		"shorter than a stamp": []byte("PBDEMS2"),
		"random bytes":         bytes.Repeat([]byte("garbage!"), 64),
		"source 1 header cut":  append([]byte("HL2DEMO\x00"), make([]byte, 40)...),
		"source 2 header cut":  append([]byte("PBDEMS2\x00"), make([]byte, 8)...),
		// A first message claiming a 4 GiB header must not be allocated.
		"source 2 huge header": append(append([]byte("PBDEMS2\x00"), make([]byte, 8)...), 0x01, 0x00, 0xff, 0xff, 0xff, 0xff, 0x0f),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			demo, err := GetDemoFromPath(writeFile(t, "broken.dem", content))
			if demo != nil || !errors.Is(err, ErrInvalidDemo) {
				t.Fatalf("demo=%v err=%v, want ErrInvalidDemo", demo, err)
			}
		})
	}
}

func TestGetDemoFromPathMissingFile(t *testing.T) {
	if _, err := GetDemoFromPath(filepath.Join(t.TempDir(), "missing.dem")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestContentHashCoversWholeFile(t *testing.T) {
	ctx := context.Background()
	base := bytes.Repeat([]byte("0123456789abcdef"), 1<<14)
	first, err := ContentHash(ctx, writeFile(t, "a.dem", base))
	if err != nil {
		t.Fatal(err)
	}
	// Same bytes under another name: same identity.
	again, _ := ContentHash(ctx, writeFile(t, "renamed.dem", base))
	if first != again {
		t.Fatal("identical content produced different hashes")
	}
	// One byte changed deep inside the file, same length: different identity.
	changed := append([]byte(nil), base...)
	changed[len(changed)/2] ^= 0xff
	other, _ := ContentHash(ctx, writeFile(t, "b.dem", changed))
	if first == other {
		t.Fatal("a modified byte in the middle of the file did not change the hash")
	}
	if len(first) != 64 {
		t.Fatalf("hash %q is not a hex SHA-256", first)
	}
}

func TestContentHashStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ContentHash(ctx, writeFile(t, "a.dem", make([]byte, 1<<20))); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
}
