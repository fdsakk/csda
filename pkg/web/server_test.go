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
