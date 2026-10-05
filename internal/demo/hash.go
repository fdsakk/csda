package demo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// ContentHash returns the hex SHA-256 of the whole file. Unlike Demo.Checksum,
// which only covers header fields and the file size, it identifies the exact
// bytes of a demo, so different matches can never share an identity. The file
// is streamed, and reading stops early when ctx is cancelled.
func ContentHash(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, &contextReader{ctx: ctx, reader: file}); err != nil {
		return "", fmt.Errorf("hashing %q: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
