package ui

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"graphdb/internal/embedding"
	"graphdb/internal/query"
)

//go:embed web/*
var webFiles embed.FS

// Server Struct:
type Server struct {
	provider query.GraphProvider
	embedder embedding.Embedder
	mux      *http.ServeMux
}

// NewServer Constructor:
func NewServer(p query.GraphProvider, e embedding.Embedder) *Server {
	s := &Server{
		provider: p,
		embedder: e,
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth())
	s.mux.HandleFunc("/api/query", s.handleQuery())

	// Serve embedded static files
	staticFS, _ := fs.Sub(webFiles, "web")
	s.mux.Handle("/", http.FileServer(http.FS(staticFS)))
}
func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func (s *Server) handleQuery() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req QueryRequest
		if r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				s.error(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
		} else {
			req.Type = r.URL.Query().Get("type")
			req.Target = r.URL.Query().Get("target")
			req.Target2 = r.URL.Query().Get("target2")
			req.Module = r.URL.Query().Get("module")
			req.Layer = r.URL.Query().Get("layer")
			req.EdgeTypes = r.URL.Query().Get("edge-types")
			req.Direction = r.URL.Query().Get("direction")

			if d := r.URL.Query().Get("depth"); d != "" {
				req.Depth, _ = strconv.Atoi(d)
			}
			if req.Depth == 0 {
				req.Depth = 1
			}

			if l := r.URL.Query().Get("limit"); l != "" {
				req.Limit, _ = strconv.Atoi(l)
			}
			if req.Limit == 0 {
				req.Limit = 10
			}

			if s := r.URL.Query().Get("similarity"); s != "" {
				req.Similarity, _ = strconv.ParseFloat(s, 64)
			}
			if req.Similarity == 0 {
				req.Similarity = 0.5
			}
		}

		if req.Type == "" {
			s.error(w, "Missing query type", http.StatusBadRequest)
			return
		}

		var result interface{}
		var err error

		switch req.Type {
		case "features", "search-features":
			if req.Target == "" {
				s.error(w, "Missing target for search-features query", http.StatusBadRequest)
				return
			}
			if s.embedder == nil {
				s.error(w, "Semantic search is disabled (no embedder)", http.StatusInternalServerError)
				return
			}
			embeddings, err := s.embedder.EmbedBatch([]string{req.Target})
			if err != nil {
				s.error(w, "Embedding failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			result, err = s.provider.SearchFeatures(embeddings[0], req.Limit)
		case "search-similar":
			if req.Target == "" {
				s.error(w, "Missing target for search-similar query", http.StatusBadRequest)
				return
			}
			if s.embedder == nil {
				s.error(w, "Semantic search is disabled (no embedder)", http.StatusInternalServerError)
				return
			}
			embeddings, err := s.embedder.EmbedBatch([]string{req.Target})
			if err != nil {
				s.error(w, "Embedding failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			result, err = s.provider.SearchSimilarFunctions(embeddings[0], req.Limit)
		case "hybrid-context":
			if req.Target == "" {
				s.error(w, "Missing target for hybrid-context query", http.StatusBadRequest)
				return
			}
			neighbors, err := s.provider.GetNeighbors(req.Target, req.Depth)
			if err != nil {
				s.error(w, "Neighbors lookup failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			var similar []*query.FeatureResult
			if s.embedder != nil {
				embeddings, err := s.embedder.EmbedBatch([]string{req.Target})
				if err == nil && len(embeddings) > 0 {
					similar, _ = s.provider.SearchSimilarFunctions(embeddings[0], req.Limit)
				}
			}
			result = map[string]interface{}{
				"neighbors": neighbors,
				"similar":   similar,
			}
		case "test-context", "neighbors":
			if req.Target == "" {
				s.error(w, "Missing target for neighbors query", http.StatusBadRequest)
				return
			}
			if req.Depth == 0 {
				req.Depth = 1
			}
			result, err = s.provider.GetNeighbors(req.Target, req.Depth)
		case "impact":
			if req.Target == "" {
				s.error(w, "Missing target for impact query", http.StatusBadRequest)
				return
			}
			result, err = s.provider.GetImpact(req.Target, req.Depth)
		case "globals":
			if req.Target == "" {
				s.error(w, "Missing target for globals query", http.StatusBadRequest)
				return
			}
			result, err = s.provider.GetGlobals(req.Target)
		case "coverage":
			if req.Target == "" {
				s.error(w, "Missing target for coverage query", http.StatusBadRequest)
				return
			}
			result, err = s.provider.GetCoverage(req.Target)
		case "seams":
			result, err = s.provider.GetSeams(req.Module, req.Layer)
		case "hotspots":
			result, err = s.provider.GetHotspots(req.Module)
		case "locate-usage":
			if req.Target == "" || req.Target2 == "" {
				s.error(w, "Missing target or target2 for locate-usage query", http.StatusBadRequest)
				return
			}
			result, err = s.provider.LocateUsage(req.Target, req.Target2)
		case "fetch-source":
			if req.Target == "" {
				s.error(w, "Missing target for fetch-source query", http.StatusBadRequest)
				return
			}
			result, err = s.provider.FetchSource(req.Target)
		case "explore-domain":
			if req.Target == "" {
				s.error(w, "Missing target for explore-domain query", http.StatusBadRequest)
				return
			}
			result, err = s.provider.ExploreDomain(req.Target)
		case "what-if":
			if req.Target == "" {
				s.error(w, "Missing target for what-if query", http.StatusBadRequest)
				return
			}
			targets := strings.Split(req.Target, ",")
			if req.Target2 != "" {
				targets = append(targets, req.Target2)
			}
			var cleanTargets []string
			for _, t := range targets {
				trimmed := strings.TrimSpace(t)
				if trimmed != "" {
					cleanTargets = append(cleanTargets, trimmed)
				}
			}
			result, err = s.provider.WhatIf(cleanTargets)
		case "traverse":
			if req.Target == "" {
				s.error(w, "Missing target for traverse query", http.StatusBadRequest)
				return
			}
			dir := query.Outgoing
			switch strings.ToLower(req.Direction) {
			case "incoming":
				dir = query.Incoming
			case "both":
				dir = query.Both
			}
			result, err = s.provider.Traverse(req.Target, req.EdgeTypes, dir, req.Depth)
		case "semantic-trace":
			if req.Target == "" {
				s.error(w, "Missing target for semantic-trace query", http.StatusBadRequest)
				return
			}
			result, err = s.provider.SemanticTrace(req.Target)
		case "overview":
			result, err = s.provider.GetOverview()
		case "status":
			commit, err := s.provider.GetGraphState()
			if err != nil {
				s.error(w, "Status check failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			result = map[string]string{
				"commit": commit,
			}
		case "semantic-seams":
			result, err = s.provider.GetSemanticSeams(r.Context(), req.Similarity)
		default:
			s.error(w, "Unsupported query type: "+req.Type, http.StatusBadRequest)
			return
		}

		if err != nil {
			s.error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func (s *Server) error(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// QueryRequest maps to the expected parameters for the /api/query endpoint.
type QueryRequest struct {
	Type       string  `json:"type"`
	Target     string  `json:"target"`
	Target2    string  `json:"target2,omitempty"`
	Depth      int     `json:"depth,omitempty"`
	Limit      int     `json:"limit,omitempty"`
	Module     string  `json:"module,omitempty"`
	Layer      string  `json:"layer,omitempty"`
	EdgeTypes  string  `json:"edge-types,omitempty"`
	Direction  string  `json:"direction,omitempty"`
	Similarity float64 `json:"similarity,omitempty"`
}

// ErrorResponse normalizes API errors.
type ErrorResponse struct {
	Error string `json:"error"`
}
