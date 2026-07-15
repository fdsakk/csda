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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akiver/cs-demo-analyzer/pkg/api"
	"github.com/akiver/cs-demo-analyzer/pkg/api/constants"
)

const maxUploadBytes int64 = 8 << 30

type uploadCollector struct {
	dir   string
	bytes int64
	paths []string
	names []string
}

func (uploads *uploadCollector) addDemo(name string, reader io.Reader) error {
	filename := filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if filename == "." || filename == "" || !strings.EqualFold(filepath.Ext(filename), ".dem") {
		return fmt.Errorf("%s is not a .dem file", name)
	}

	remaining := maxUploadBytes - uploads.bytes
	if remaining <= 0 {
		return fmt.Errorf("demos exceed the %d GiB upload limit", maxUploadBytes>>30)
	}
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	extension := filepath.Ext(filename)
	for index := 0; ; index++ {
		candidate := filename
		if index > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", base, index, extension)
		}
		path := filepath.Join(uploads.dir, candidate)
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(file, io.LimitReader(reader, remaining+1))
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil || written > remaining {
			_ = os.Remove(path)
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			return fmt.Errorf("demos exceed the %d GiB upload limit", maxUploadBytes>>30)
		}
		uploads.bytes += written
		uploads.paths = append(uploads.paths, path)
		uploads.names = append(uploads.names, candidate)
		return nil
	}
}

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
	// Progress is the overall job progress in percent, including partial
	// parse progress of demos still being analyzed.
	Progress float64 `json:"progress"`
	paths    []string
	source   constants.DemoSource
}

type Server struct {
	options Options
	mux     *http.ServeMux
	mu      sync.RWMutex
	jobs    map[string]*Job
	queue   chan string
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	close   sync.Once
}

func NewServer(options Options) (*Server, error) {
	if options.DatabasePath == "" {
		options.DatabasePath = "player-stats.db"
	}
	if options.UploadsPath == "" {
		options.UploadsPath = "uploads"
	}
	if err := os.MkdirAll(options.UploadsPath, 0o755); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{options: options, mux: http.NewServeMux(), jobs: make(map[string]*Job), queue: make(chan string, 32), ctx: ctx, cancel: cancel, done: make(chan struct{})}
	server.routes()
	go func() {
		defer close(server.done)
		server.worker()
	}()
	return server, nil
}

func (s *Server) Close() {
	s.close.Do(func() {
		s.cancel()
		<-s.done
	})
}
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/report", s.handleReport)
	s.mux.HandleFunc("GET /api/jobs", s.handleJobs)
	s.mux.HandleFunc("POST /api/uploads", s.handleUpload)
	s.mux.HandleFunc("PATCH /api/demos/{checksum}", s.handleDemoToggle)
	s.mux.HandleFunc("DELETE /api/demos/{checksum}", s.handleDemoDelete)
	s.mux.HandleFunc("PATCH /api/players/{steamId}", s.handlePlayerSaved)
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
	uploads := uploadCollector{dir: dir}
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
		var uploadErr error
		if strings.EqualFold(filepath.Ext(filename), ".dem") {
			uploadErr = uploads.addDemo(filename, part)
		} else {
			uploadErr = fmt.Errorf("%s is not a .dem file", filename)
		}
		part.Close()
		if uploadErr != nil {
			cleanup()
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": uploadErr.Error()})
			return
		}
	}
	if len(uploads.paths) == 0 {
		cleanup()
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no .dem files uploaded"})
		return
	}
	source := constants.DemoSource(r.URL.Query().Get("source"))
	if source == "" {
		source = s.options.Source
	}
	if source != "" {
		if err := api.ValidateDemoSource(source); err != nil {
			cleanup()
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}
	job := &Job{ID: id, Status: JobQueued, Files: uploads.names, CreatedAt: time.Now().UTC(), Total: len(uploads.paths), paths: uploads.paths, source: source}
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

func (s *Server) handleDemoDelete(w http.ResponseWriter, r *http.Request) {
	demoPath, err := api.DeleteDemo(r.Context(), s.options.DatabasePath, r.PathValue("checksum"))
	if errors.Is(err, api.ErrDemoNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "demo not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Remove the uploaded file too, but only when it actually lives inside
	// this server's uploads folder (imported rows may reference foreign paths).
	if uploadsRoot, absErr := filepath.Abs(s.options.UploadsPath); absErr == nil {
		if absPath, pathErr := filepath.Abs(demoPath); pathErr == nil {
			if relative, relErr := filepath.Rel(uploadsRoot, absPath); relErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				_ = os.Remove(absPath)
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePlayerSaved(w http.ResponseWriter, r *http.Request) {
	steamID, err := strconv.ParseUint(r.PathValue("steamId"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid steam id"})
		return
	}
	var body struct {
		Saved  *bool `json:"saved"`
		Banned *bool `json:"banned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Saved == nil && body.Banned == nil) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": `expected JSON body with a "saved" or "banned" boolean`})
		return
	}
	if body.Saved != nil {
		err = api.SetPlayerSaved(r.Context(), s.options.DatabasePath, steamID, *body.Saved)
	}
	if err == nil && body.Banned != nil {
		err = api.SetPlayerBanned(r.Context(), s.options.DatabasePath, steamID, *body.Banned)
	}
	if errors.Is(err, api.ErrPlayerNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "player not found"})
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
		if s.ctx.Err() != nil {
			return
		}
		select {
		case <-s.ctx.Done():
			return
		case id := <-s.queue:
			if s.ctx.Err() != nil {
				return
			}
			s.mu.Lock()
			job := s.jobs[id]
			now := time.Now().UTC()
			job.Status = JobRunning
			job.StartedAt = &now
			s.mu.Unlock()
			fractions := make(map[string]float64, len(job.paths))
			total := len(job.paths)
			result, err := api.BuildPlayerStatsDatabase(s.ctx, api.PlayerStatsBuildOptions{
				DatabasePath:                s.options.DatabasePath,
				DemoPaths:                   job.paths,
				Source:                      job.source,
				Jobs:                        4,
				VisibilityConfirmationTicks: 3,
				OnDemoProcessed: func(processed, total int) {
					s.mu.Lock()
					job.Processed = processed
					job.Total = total
					s.mu.Unlock()
				},
				// Every demo ends at fraction 1 (finished or skipped), so the
				// sum over all demos is the true overall progress.
				OnDemoProgress: func(path string, fraction float64) {
					s.mu.Lock()
					fractions[path] = fraction
					sum := 0.0
					for _, f := range fractions {
						sum += f
					}
					if total > 0 {
						job.Progress = 100 * sum / float64(total)
					}
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
