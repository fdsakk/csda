package web

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz/lzma"
)

func TestHealthAndEmptyReport(t *testing.T) {
	root := t.TempDir()
	server, err := NewServer(Options{DatabasePath: filepath.Join(root, "stats.db"), UploadsPath: filepath.Join(root, "uploads")})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	for _, path := range []string{"/api/health", "/api/report"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestUploadRejectsNonDemo(t *testing.T) {
	root := t.TempDir()
	server, err := NewServer(Options{DatabasePath: filepath.Join(root, "stats.db"), UploadsPath: filepath.Join(root, "uploads")})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("demos", "notes.txt")
	_, _ = part.Write([]byte("no"))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestUploadEnqueuesDemoWithAutomaticSource(t *testing.T) {
	root := t.TempDir()
	server, err := NewServer(Options{DatabasePath: filepath.Join(root, "stats.db"), UploadsPath: filepath.Join(root, "uploads")})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("demos", "sample.dem")
	_, _ = part.Write([]byte("not a real demo"))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var job Job
	if err := json.Unmarshal(response.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job.ID == "" || len(job.Files) != 1 {
		t.Fatalf("bad job: %+v", job)
	}
	server.mu.RLock()
	source := server.jobs[job.ID].source
	server.mu.RUnlock()
	if source != "" {
		t.Fatalf("source=%q, want automatic detection", source)
	}
}

func zipBytes(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var body bytes.Buffer
	archive := zip.NewWriter(&body)
	for name, contents := range entries {
		file, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func zstdZIPBytes(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var body bytes.Buffer
	archive := zip.NewWriter(&body)
	archive.RegisterCompressor(zstd.ZipMethodWinZip, zstd.ZipCompressor())
	for name, contents := range entries {
		file, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zstd.ZipMethodWinZip})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

type lzmaZIPCompressor struct {
	destination io.Writer
	input       bytes.Buffer
}

func (compressor *lzmaZIPCompressor) Write(data []byte) (int, error) {
	return compressor.input.Write(data)
}

func (compressor *lzmaZIPCompressor) Close() error {
	var encoded bytes.Buffer
	writer, err := lzma.NewWriter(&encoded)
	if err != nil {
		return err
	}
	if _, err = writer.Write(compressor.input.Bytes()); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	stream := encoded.Bytes()
	if len(stream) < lzma.HeaderLen {
		return io.ErrUnexpectedEOF
	}
	_, err = io.Copy(compressor.destination, io.MultiReader(
		bytes.NewReader([]byte{0x10, 0x02, zipLZMAPropertiesSize, 0x00}),
		bytes.NewReader(stream[:zipLZMAPropertiesSize]),
		bytes.NewReader(stream[lzma.HeaderLen:]),
	))
	return err
}

func lzmaZIPBytes(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var body bytes.Buffer
	archive := zip.NewWriter(&body)
	archive.RegisterCompressor(zipMethodLZMA, func(destination io.Writer) (io.WriteCloser, error) {
		return &lzmaZIPCompressor{destination: destination}, nil
	})
	for name, contents := range entries {
		file, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zipMethodLZMA})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func TestUploadExtractsDemosFromZIP(t *testing.T) {
	root := t.TempDir()
	server, err := NewServer(Options{DatabasePath: filepath.Join(root, "stats.db"), UploadsPath: filepath.Join(root, "uploads")})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("demos", "match-demos.zip")
	_, _ = part.Write(zipBytes(t, map[string]string{
		"nested/match.dem":  "not a real demo",
		"nested/readme.txt": "ignore me",
	}))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var job Job
	if err := json.Unmarshal(response.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if len(job.Files) != 1 || job.Files[0] != "match.dem" {
		t.Fatalf("files=%v, want [match.dem]", job.Files)
	}
}

func TestUploadExtractsDemosFromZstandardZIP(t *testing.T) {
	server := newTestServer(t)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("demos", "match-demos.zip")
	_, _ = part.Write(zstdZIPBytes(t, map[string]string{"match.dem": "not a real demo"}))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestUploadExtractsDemosFromLZMAZIP(t *testing.T) {
	server := newTestServer(t)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("demos", "match-demos.zip")
	_, _ = part.Write(lzmaZIPBytes(t, map[string]string{"match.dem": "not a real demo"}))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestUploadRejectsZIPWithoutDemos(t *testing.T) {
	server := newTestServer(t)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("demos", "notes.zip")
	_, _ = part.Write(zipBytes(t, map[string]string{"notes.txt": "no demo here"}))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	server, err := NewServer(Options{DatabasePath: filepath.Join(root, "stats.db"), UploadsPath: filepath.Join(root, "uploads")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	return server
}

func do(server *Server, method, path string, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	return response
}

const validExport = `{"format":"cs-demo-analyzer/player-stats","version":1,"exportedAt":"2026-07-13T00:00:00Z","players":[{"steamId":"76561198000000001","latestName":"Alice","names":["Alice"]}],"demos":[{"checksum":"web1","path":"a.dem","fileName":"a","mapName":"de_test","demoDate":"2026-01-01T00:00:00Z","tickRate":64,"buildNumber":1,"source":"valve","analysisVersion":1,"importedAt":"2026-07-13T00:00:00Z","playerStats":[{"steamId":"76561198000000001","rounds":10,"shots":50,"hitShots":25}],"encounters":[],"reactions":[],"weaponStats":[],"evidence":[]}]}`

func TestImportToggleExportFlow(t *testing.T) {
	server := newTestServer(t)

	response := do(server, http.MethodPost, "/api/import", []byte(validExport))
	if response.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct{ Imported, Skipped int }
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Imported != 1 || result.Skipped != 0 {
		t.Fatalf("result=%+v", result)
	}

	response = do(server, http.MethodPatch, "/api/demos/web1", []byte(`{"enabled":false}`))
	if response.Code != http.StatusNoContent {
		t.Fatalf("toggle status=%d body=%s", response.Code, response.Body.String())
	}

	response = do(server, http.MethodGet, "/api/export", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("export status=%d", response.Code)
	}
	if got := response.Header().Get("Content-Disposition"); got == "" {
		t.Fatal("missing Content-Disposition")
	}
	var export struct{ Demos []struct{ Checksum string } }
	if err := json.Unmarshal(response.Body.Bytes(), &export); err != nil {
		t.Fatal(err)
	}
	if len(export.Demos) != 0 {
		t.Fatalf("disabled demo exported: %+v", export.Demos)
	}
}

func TestToggleValidation(t *testing.T) {
	server := newTestServer(t)
	if response := do(server, http.MethodPatch, "/api/demos/missing", []byte(`{"enabled":true}`)); response.Code != http.StatusNotFound {
		t.Fatalf("status=%d", response.Code)
	}
	if response := do(server, http.MethodPatch, "/api/demos/missing", []byte(`not json`)); response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestImportValidation(t *testing.T) {
	server := newTestServer(t)
	if response := do(server, http.MethodPost, "/api/import", []byte(`{"format":"wrong","version":1}`)); response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
