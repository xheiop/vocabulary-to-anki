// Package server exposes the local HTTP intake used by the browser userscript:
// POST /add enqueues a word, GET /health is a liveness probe. Requests are
// handed to a channel and answered immediately so the browser never waits on
// the LLM.
package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/xheiop/vocab2anki/internal/process"
)

// Server accepts words over HTTP and forwards them to a jobs channel.
type Server struct {
	jobs chan<- process.Request
}

// New returns a server that publishes incoming words to jobs.
func New(jobs chan<- process.Request) *Server {
	return &Server{jobs: jobs}
}

// Handler builds the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/add", s.handleAdd)
	return mux
}

type addRequest struct {
	Word    string `json:"word"`
	Context string `json:"context"`
	Source  string `json:"source"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Permit the userscript (running on arbitrary origins) to probe liveness.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdd(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req addRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Word == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "word is required"})
		return
	}

	job := process.Request{Word: req.Word, Context: req.Context, Source: req.Source}
	select {
	case s.jobs <- job:
		log.Printf("intake(http): %q", req.Word)
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued", "word": req.Word})
	default:
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "busy, try again"})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
