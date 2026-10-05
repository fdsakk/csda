package web

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func postDemo(server *Server, name string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("demos", name)
	_, _ = part.Write([]byte("not a real demo, the queue does not look inside"))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	return response
}

// A server without a worker never drains its queue, so it stays full.
func TestUploadIsRejectedWhenTheQueueIsFull(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	server := &Server{
		options: Options{DatabasePath: filepath.Join(root, "stats.db"), UploadsPath: filepath.Join(root, "uploads")},
		mux:     http.NewServeMux(), jobs: make(map[string]*Job), queue: make(chan string, 1),
		subscribers: make(map[chan []byte]struct{}), ctx: ctx, cancel: cancel, done: make(chan struct{}),
	}
	if err := os.MkdirAll(server.options.UploadsPath, 0o755); err != nil {
		t.Fatal(err)
	}
	server.routes()

	if response := postDemo(server, "first.dem"); response.Code != http.StatusAccepted {
		t.Fatalf("first upload status=%d body=%s", response.Code, response.Body.String())
	}

	finished := make(chan *httptest.ResponseRecorder, 1)
	go func() { finished <- postDemo(server, "second.dem") }()
	select {
	case response := <-finished:
		if response.Code != http.StatusTooManyRequests {
			t.Fatalf("status=%d body=%s, want 429", response.Code, response.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the upload waited for a free queue slot instead of being rejected")
	}

	// the rejected upload leaves neither a job nor files behind
	if len(server.jobs) != 1 {
		t.Fatalf("%d jobs, want only the accepted one", len(server.jobs))
	}
	entries, err := os.ReadDir(server.options.UploadsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("%d upload folders, want only the accepted job's", len(entries))
	}
}

func TestBasicAuth(t *testing.T) {
	root := t.TempDir()
	server, err := NewServer(Options{
		DatabasePath: filepath.Join(root, "stats.db"), UploadsPath: filepath.Join(root, "uploads"),
		AuthUser: "alice", AuthPassword: "s3cret",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)

	get := func(path, user, password string) int {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if user != "" || password != "" {
			request.SetBasicAuth(user, password)
		}
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		return response.Code
	}

	if code := get("/api/health", "", ""); code != http.StatusOK {
		t.Errorf("health without credentials = %d, want 200", code)
	}
	for name, credentials := range map[string][2]string{
		"no credentials": {"", ""}, "wrong password": {"alice", "wrong"}, "wrong user": {"bob", "s3cret"}, "empty password": {"alice", ""},
	} {
		if code := get("/api/report", credentials[0], credentials[1]); code != http.StatusUnauthorized {
			t.Errorf("%s: report = %d, want 401", name, code)
		}
	}
	if code := get("/api/players/1/encounters", "", ""); code != http.StatusUnauthorized {
		t.Errorf("encounters without credentials = %d, want 401", code)
	}
	if code := get("/api/report", "alice", "s3cret"); code != http.StatusOK {
		t.Errorf("report with credentials = %d, want 200", code)
	}
}
