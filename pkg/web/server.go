package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	urlpath "path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/akiver/cs-demo-analyzer/pkg/api"
	"github.com/akiver/cs-demo-analyzer/pkg/api/constants"
)

const maxUploadBytes int64 = 8 << 30

type Options struct {
	DatabasePath string
	UploadsPath  string
	AssetsPath   string
	Source       constants.DemoSource
}

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
)

type Job struct {
	ID        string                      `json:"id"`
	Status    JobStatus                   `json:"status"`
	Files     []string                    `json:"files"`
	CreatedAt time.Time                   `json:"createdAt"`
	StartedAt *time.Time                  `json:"startedAt,omitempty"`
	EndedAt   *time.Time                  `json:"endedAt,omitempty"`
	Result    *api.PlayerStatsBuildResult `json:"result,omitempty"`
	Error     string                      `json:"error,omitempty"`
	Processed int                         `json:"processed"`
	Total     int                         `json:"total"`
	paths     []string
	source    constants.DemoSource
}

type Server struct {
	options Options
	mux     *http.ServeMux
	mu      sync.RWMutex
	jobs    map[string]*Job
	queue   chan string
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewServer(options Options) (*Server, error) {
	if options.DatabasePath == "" {
		options.DatabasePath = "player-stats.db"
	}
	if options.UploadsPath == "" {
		options.UploadsPath = "uploads"
	}
	if options.Source == "" {
		options.Source = constants.DemoSourceValve
	}
	if err := os.MkdirAll(options.UploadsPath, 0o755); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{options: options, mux: http.NewServeMux(), jobs: make(map[string]*Job), queue: make(chan string, 32), ctx: ctx, cancel: cancel}
	server.routes()
	go server.worker()
	return server, nil
}

func (s *Server) Close()                { s.cancel() }
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/report", s.handleReport)
	s.mux.HandleFunc("GET /api/jobs", s.handleJobs)
	s.mux.HandleFunc("POST /api/uploads", s.handleUpload)
	s.mux.HandleFunc("PATCH /api/demos/{checksum}", s.handleDemoToggle)
	s.mux.HandleFunc("GET /api/export", s.handleExport)
	s.mux.HandleFunc("POST /api/import", s.handleImport)
	s.mux.HandleFunc("/", s.handleStatic)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	report, err := api.GetPlayerStatsReport(r.Context(), api.PlayerStatsReportOptions{DatabasePath: s.options.DatabasePath, Config: api.DefaultSuspicionConfig()})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func publicJob(job *Job) Job {
	copy := *job
	copy.paths = nil
	return copy
}

func (s *Server) handleJobs(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	jobs := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, publicJob(job))
	}
	s.mu.RUnlock()
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	writeJSON(w, http.StatusOK, jobs)
}

func randomID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected multipart upload"})
		return
	}
	id, err := randomID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	dir := filepath.Join(s.options.UploadsPath, id)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var paths, names []string
	cleanup := func() { _ = os.RemoveAll(dir) }
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			cleanup()
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": nextErr.Error()})
			return
		}
		filename := filepath.Base(part.FileName())
		if filename == "" {
			part.Close()
			continue
		}
		if !strings.EqualFold(filepath.Ext(filename), ".dem") {
			part.Close()
			cleanup()
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("%s is not a .dem file", filename)})
			return
		}
		path := filepath.Join(dir, filename)
		file, createErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if createErr != nil {
			part.Close()
			cleanup()
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": createErr.Error()})
			return
		}
		_, copyErr := io.Copy(file, part)
		closeErr := file.Close()
		part.Close()
		if copyErr != nil || closeErr != nil {
			cleanup()
			if copyErr == nil {
				copyErr = closeErr
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": copyErr.Error()})
			return
		}
		paths = append(paths, path)
		names = append(names, filename)
	}
	if len(paths) == 0 {
		cleanup()
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no .dem files uploaded"})
		return
	}
	source := constants.DemoSource(r.URL.Query().Get("source"))
	if source == "" {
		source = s.options.Source
	}
	if err := api.ValidateDemoSource(source); err != nil {
		cleanup()
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	job := &Job{ID: id, Status: JobQueued, Files: names, CreatedAt: time.Now().UTC(), Total: len(paths), paths: paths, source: source}
	s.mu.Lock()
	s.jobs[id] = job
	s.mu.Unlock()
	select {
	case s.queue <- id:
		writeJSON(w, http.StatusAccepted, publicJob(job))
	case <-s.ctx.Done():
		cleanup()
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "server is shutting down"})
	}
}

const maxImportBytes int64 = 1 << 30

func (s *Server) handleDemoToggle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": `expected JSON body with "enabled" boolean`})
		return
	}
	err := api.SetDemoEnabled(r.Context(), s.options.DatabasePath, r.PathValue("checksum"), *body.Enabled)
	if errors.Is(err, api.ErrDemoNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "demo not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	export, err := api.ExportPlayerStatsData(r.Context(), s.options.DatabasePath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="player-stats-export.json"`)
	writeJSON(w, http.StatusOK, export)
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)
	var payload api.PlayerStatsExport
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	result, err := api.ImportPlayerStatsData(r.Context(), s.options.DatabasePath, &payload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) worker() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case id := <-s.queue:
			s.mu.Lock()
			job := s.jobs[id]
			now := time.Now().UTC()
			job.Status = JobRunning
			job.StartedAt = &now
			s.mu.Unlock()
			result, err := api.BuildPlayerStatsDatabase(s.ctx, api.PlayerStatsBuildOptions{
				DatabasePath:                s.options.DatabasePath,
				DemoPaths:                   job.paths,
				Source:                      job.source,
				Jobs:                        1,
				VisibilityConfirmationTicks: 3,
				OnDemoProcessed: func(processed, total int) {
					s.mu.Lock()
					job.Processed = processed
					job.Total = total
					s.mu.Unlock()
				},
			})
			s.mu.Lock()
			end := time.Now().UTC()
			job.EndedAt = &end
			job.Result = result
			if err != nil {
				job.Status = JobFailed
				job.Error = err.Error()
			} else if result.Failed > 0 {
				job.Status = JobFailed
				job.Error = "one or more demos failed to analyze"
			} else {
				job.Status = JobCompleted
			}
			s.mu.Unlock()
		}
	}
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if s.options.AssetsPath == "" {
		http.Error(w, "web assets are not configured", http.StatusNotFound)
		return
	}
	requested := strings.TrimPrefix(urlpath.Clean("/"+r.URL.Path), "/")
	if requested == "." || requested == "" {
		requested = "index.html"
	}
	path := filepath.Join(s.options.AssetsPath, requested)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		path = filepath.Join(s.options.AssetsPath, "index.html")
	}
	http.ServeFile(w, r, path)
}
