package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sync"
)

//go:embed templates/*
var templatesFS embed.FS

var indexTmpl = template.Must(template.ParseFS(templatesFS, "templates/index.html"))

// Server wraps HTTP server with benchmark runner
type Server struct {
	addr      string
	runner    *BenchmarkRunner
	report    *BenchmarkReport // For static report viewing
	staticDir string           // Directory for static files (html/matchtest)
	mux       *http.ServeMux
}

// NewServer creates a server for live benchmark progress
func NewServer(addr string, runner *BenchmarkRunner, staticDir string) *Server {
	s := &Server{
		addr:      addr,
		runner:    runner,
		staticDir: staticDir,
		mux:       http.NewServeMux(),
	}
	s.setupRoutes()
	return s
}

// NewServerWithReport creates a server for viewing static report
func NewServerWithReport(addr string, report *BenchmarkReport, staticDir string) *Server {
	s := &Server{
		addr:      addr,
		report:    report,
		staticDir: staticDir,
		mux:       http.NewServeMux(),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// API endpoints
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/progress", s.handleProgress)
	s.mux.HandleFunc("/api/report", s.handleReport)

	// SSE for real-time updates
	s.mux.HandleFunc("/api/events", s.handleSSE)

	// Static files or embedded template
	if s.staticDir != "" {
		// Serve static files from directory
		if _, err := os.Stat(s.staticDir); err == nil {
			s.mux.Handle("/", http.FileServer(http.Dir(s.staticDir)))
		} else {
			fmt.Printf("Warning: static dir not found: %s, using embedded template\n", s.staticDir)
			s.mux.HandleFunc("/", s.handleIndex)
		}
	} else {
		// Use embedded template
		s.mux.HandleFunc("/", s.handleIndex)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get report data for template
	var report *BenchmarkReport
	if s.runner != nil {
		report = s.runner.GetReport()
	} else {
		report = s.report
	}

	if report == nil {
		report = &BenchmarkReport{}
	}

	if err := indexTmpl.Execute(w, report); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.runner != nil {
		status := s.runner.GetStatus()
		json.NewEncoder(w).Encode(status)
	} else {
		json.NewEncoder(w).Encode(map[string]string{"status": "static"})
	}
}

func (s *Server) handleProgress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.runner != nil {
		progress := s.runner.GetProgress()
		json.NewEncoder(w).Encode(map[string]any{
			"status":   s.runner.GetStatus(),
			"models":   progress,
		})
	} else {
		json.NewEncoder(w).Encode(map[string]string{"status": "static"})
	}
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var report *BenchmarkReport
	if s.runner != nil {
		report = s.runner.GetReport()
	} else {
		report = s.report
	}

	if report == nil {
		report = &BenchmarkReport{}
	}

	json.NewEncoder(w).Encode(report)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	if s.runner == nil {
		http.Error(w, "No active benchmark", http.StatusBadRequest)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Subscribe to updates
	ch := s.runner.Subscribe()
	defer s.runner.Unsubscribe(ch)

	// Send initial state
	initial := ProgressUpdate{
		Type:   "init",
		Status: func() *RunnerStatus { s := s.runner.GetStatus(); return &s }(),
		Models: s.runner.GetProgress(),
	}
	data, _ := json.Marshal(initial)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	// Stream updates
	for {
		select {
		case <-r.Context().Done():
			return
		case update, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(update)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	fmt.Printf("Server started at http://%s\n", s.addr)
	fmt.Println("  - GET /           Web UI")
	fmt.Println("  - GET /api/status  Current status")
	fmt.Println("  - GET /api/progress Progress for all models")
	fmt.Println("  - GET /api/report   Full report (JSON)")
	fmt.Println("  - GET /api/events   SSE progress stream")
	fmt.Println()
	return http.ListenAndServe(s.addr, s.mux)
}

// StartAsync starts the server in background
func (s *Server) StartAsync(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Start(); err != nil {
			fmt.Printf("Server error: %v\n", err)
		}
	}()
}

// Legacy function for backward compatibility
func startServer(addr string, report *BenchmarkReport, staticDir string) error {
	s := NewServerWithReport(addr, report, staticDir)
	return s.Start()
}
