package web

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
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

func TestUploadEnqueuesDemo(t *testing.T) {
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
	request := httptest.NewRequest(http.MethodPost, "/api/uploads?source=valve", &body)
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
