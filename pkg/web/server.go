package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	urlpath "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fdsakk/csda/pkg/api"
	"github.com/fdsakk/csda/pkg/api/constants"
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
	// Assets contains the dashboard files rooted at index.html. It takes
	// precedence over AssetsPath and is normally backed by go:embed.
	Assets fs.FS
	// AssetsPath is retained as an override for local UI development.
	AssetsPath string
	Source     constants.DemoSource
	// Config holds the suspicion thresholds used by /api/report. Zero value
	// falls back to api.DefaultSuspicionConfig().
	Config api.SuspicionConfig
	// AuthUser/AuthPassword enable HTTP Basic Auth on everything except
	// /api/health when both are set.
	AuthUser     string
	AuthPassword string
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
	options     Options
	mux         *http.ServeMux
	mu          sync.RWMutex
	configMu    sync.RWMutex
	uploadsMu   sync.Mutex
	jobs        map[string]*Job
	queue       chan string
	subMu       sync.Mutex
	subscribers map[chan []byte]struct{}
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	close       sync.Once
}

func NewServer(options Options) (*Server, error) {
	if options.DatabasePath == "" {
		options.DatabasePath = "player-stats.db"
	}
	if options.UploadsPath == "" {
		options.UploadsPath = "uploads"
	}
	if options.Config.MinimumDemos == 0 {
		options.Config = api.DefaultSuspicionConfig()
	}
	// A persisted config overrides the startup default so thresholds edited in
	// the UI survive restarts.
	if config, ok, err := api.GetThresholds(context.Background(), options.DatabasePath); err != nil {
		return nil, err
	} else if ok {
		options.Config = config
	}
	if err := os.MkdirAll(options.UploadsPath, 0o755); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{options: options, mux: http.NewServeMux(), jobs: make(map[string]*Job), queue: make(chan string, 32), subscribers: make(map[chan []byte]struct{}), ctx: ctx, cancel: cancel, done: make(chan struct{})}
	server.routes()
	go server.broadcastJobs()
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

// Handler returns the HTTP handler, wrapped with Basic Auth when credentials
// are configured. /api/health stays open for container health checks.
func (s *Server) Handler() http.Handler {
	if s.options.AuthUser == "" && s.options.AuthPassword == "" {
		return s.mux
	}
	wantUser := sha256.Sum256([]byte(s.options.AuthUser))
	wantPassword := sha256.Sum256([]byte(s.options.AuthPassword))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			s.mux.ServeHTTP(w, r)
			return
		}
		user, password, ok := r.BasicAuth()
		gotUser := sha256.Sum256([]byte(user))
		gotPassword := sha256.Sum256([]byte(password))
		if !ok || subtle.ConstantTimeCompare(wantUser[:], gotUser[:]) != 1 || subtle.ConstantTimeCompare(wantPassword[:], gotPassword[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="cs-demo-analyzer", charset="UTF-8"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/report", s.handleReport)
	s.mux.HandleFunc("GET /api/thresholds", s.handleThresholdsGet)
	s.mux.HandleFunc("PUT /api/thresholds", s.handleThresholdsPut)
	s.mux.HandleFunc("GET /api/jobs", s.handleJobs)
	s.mux.HandleFunc("GET /api/events", s.handleEvents)
	s.mux.HandleFunc("DELETE /api/jobs/{id}", s.handleJobDelete)
	s.mux.HandleFunc("POST /api/uploads", s.handleUpload)
	s.mux.HandleFunc("DELETE /api/uploads", s.handleUploadsClear)
	s.mux.HandleFunc("PATCH /api/demos", s.handleAllDemosToggle)
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
	s.configMu.RLock()
	config := s.options.Config
	s.configMu.RUnlock()
	report, err := api.GetPlayerStatsReport(r.Context(), api.PlayerStatsReportOptions{DatabasePath: s.options.DatabasePath, Config: config})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleThresholdsGet(w http.ResponseWriter, _ *http.Request) {
	s.configMu.RLock()
	config := s.options.Config
	s.configMu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]api.SuspicionConfig{
		"current":  config,
		"defaults": api.DefaultSuspicionConfig(),
	})
}

func (s *Server) handleThresholdsPut(w http.ResponseWriter, r *http.Request) {
	var config api.SuspicionConfig
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid thresholds: " + err.Error()})
		return
	}
	if err := api.ValidateSuspicionConfig(config); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := api.SaveThresholds(r.Context(), s.options.DatabasePath, config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.configMu.Lock()
	s.options.Config = config
	s.configMu.Unlock()
	writeJSON(w, http.StatusOK, config)
}

func publicJob(job *Job) Job {
	copy := *job
	copy.paths = nil
	return copy
}

func (s *Server) sortedJobs() []Job {
	s.mu.RLock()
	jobs := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, publicJob(job))
	}
	s.mu.RUnlock()
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	return jobs
}

func (s *Server) handleJobs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.sortedJobs())
}

func (s *Server) handleJobDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	job, ok := s.jobs[id]
	if ok && (job.Status == JobQueued || job.Status == JobRunning) {
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "job is still running"})
		return
	}
	delete(s.jobs, id)
	s.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// broadcastJobs pushes the job list to SSE subscribers whenever it changes,
// checking at a fixed cadence so per-tick parse progress can't flood clients.
func (s *Server) broadcastJobs() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var last []byte
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			payload, err := json.Marshal(s.sortedJobs())
			if err != nil || bytes.Equal(payload, last) {
				continue
			}
			last = payload
			s.subMu.Lock()
			for subscriber := range s.subscribers {
				select {
				case subscriber <- payload:
				default: // slow client; it catches up on the next change
				}
			}
			s.subMu.Unlock()
		}
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	subscriber := make(chan []byte, 4)
	s.subMu.Lock()
	s.subscribers[subscriber] = struct{}{}
	s.subMu.Unlock()
	defer func() {
		s.subMu.Lock()
		delete(s.subscribers, subscriber)
		s.subMu.Unlock()
	}()

	send := func(payload []byte) bool {
		_, err := fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
		return err == nil
	}
	if payload, err := json.Marshal(s.sortedJobs()); err == nil && !send(payload) {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.ctx.Done():
			return
		case payload := <-subscriber:
			if !send(payload) {
				return
			}
		}
	}
}

func randomID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	s.uploadsMu.Lock()
	defer s.uploadsMu.Unlock()
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

func (s *Server) handleUploadsClear(w http.ResponseWriter, _ *http.Request) {
	s.uploadsMu.Lock()
	defer s.uploadsMu.Unlock()

	s.mu.RLock()
	for _, job := range s.jobs {
		if job.Status == JobQueued || job.Status == JobRunning {
			s.mu.RUnlock()
			writeJSON(w, http.StatusConflict, map[string]string{"error": "wait for demo analysis to finish before clearing uploads"})
			return
		}
	}
	s.mu.RUnlock()

	root, err := filepath.Abs(s.options.UploadsPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	volumeRoot := filepath.Clean(filepath.VolumeName(root) + string(filepath.Separator))
	workingDirectory, _ := os.Getwd()
	if filepath.Clean(root) == volumeRoot || (workingDirectory != "" && filepath.Clean(root) == filepath.Clean(workingDirectory)) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "refusing to clear an unsafe uploads path"})
		return
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		err = os.MkdirAll(root, 0o755)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

const maxImportBytes int64 = 1 << 30

func (s *Server) handleAllDemosToggle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": `expected JSON body with "enabled" boolean`})
		return
	}
	if err := api.SetAllDemosEnabled(r.Context(), s.options.DatabasePath, *body.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

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
			s.pruneFinishedLocked()
			s.mu.Unlock()
			// The uploaded .dem files are only needed during analysis — the
			// stats live in the database afterwards. Re-analyzing after an
			// algorithm change requires re-uploading the demo.
			_ = os.RemoveAll(filepath.Join(s.options.UploadsPath, id))
		}
	}
}

// maxFinishedJobs caps how many completed/failed jobs stay listed in memory.
const maxFinishedJobs = 20

func (s *Server) pruneFinishedLocked() {
	finished := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		if job.Status == JobCompleted || job.Status == JobFailed {
			finished = append(finished, job)
		}
	}
	if len(finished) <= maxFinishedJobs {
		return
	}
	sort.Slice(finished, func(i, j int) bool { return finished[i].CreatedAt.After(finished[j].CreatedAt) })
	for _, job := range finished[maxFinishedJobs:] {
		delete(s.jobs, job.ID)
	}
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	assets := s.options.Assets
	if assets == nil && s.options.AssetsPath != "" {
		assets = os.DirFS(s.options.AssetsPath)
	}
	if assets == nil {
		http.Error(w, "web assets are not configured", http.StatusNotFound)
		return
	}
	requested := strings.TrimPrefix(urlpath.Clean("/"+r.URL.Path), "/")
	if requested == "." || requested == "" {
		requested = "index.html"
	}
	if !fs.ValidPath(requested) {
		http.NotFound(w, r)
		return
	}
	name := requested
	info, err := fs.Stat(assets, name)
	if err != nil || info.IsDir() {
		// Asset requests should fail visibly instead of receiving HTML with a
		// JavaScript content type. Other paths are client-side React routes.
		if strings.HasPrefix(requested, "assets/") {
			http.NotFound(w, r)
			return
		}
		name = "index.html"
	}
	data, err := fs.ReadFile(assets, name)
	if err != nil {
		http.Error(w, "web UI is not included in this build", http.StatusNotFound)
		return
	}
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
