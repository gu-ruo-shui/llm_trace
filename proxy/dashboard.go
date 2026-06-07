package proxy

import (
	"embed"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

//go:embed dashboard_static/*
var dashboardStatic embed.FS

type DashboardHandler struct {
	logger    *DatabaseLogger
	targetURL string
	dbPath    string
}

func NewDashboardHandler(logger *DatabaseLogger, targetURL, dbPath string) *DashboardHandler {
	return &DashboardHandler{logger: logger, targetURL: targetURL, dbPath: dbPath}
}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/_ui")
	if path == "" || path == "/" {
		serveDashboardAsset(w, r, "index.html")
		return
	}

	if !strings.HasPrefix(path, "/api/") {
		serveDashboardAsset(w, r, strings.TrimPrefix(path, "/"))
		return
	}

	if h.logger == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error": "dashboard API needs database logging; start with USE_DB=true",
		})
		return
	}

	switch {
	case path == "/api/health":
		h.handleHealth(w)
	case path == "/api/logs":
		h.handleLogs(w, r)
	case path == "/api/stats":
		h.handleStats(w)
	case strings.HasPrefix(path, "/api/logs/") && strings.HasSuffix(path, "/events"):
		uuid := strings.TrimSuffix(strings.TrimPrefix(path, "/api/logs/"), "/events")
		h.handleEvents(w, uuid)
	case strings.HasPrefix(path, "/api/logs/"):
		uuid := strings.TrimPrefix(path, "/api/logs/")
		h.handleLog(w, uuid)
	default:
		http.NotFound(w, r)
	}
}

func (h *DashboardHandler) handleHealth(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"target_url": h.targetURL,
		"db_path":    h.dbPath,
	})
}

func (h *DashboardHandler) handleStats(w http.ResponseWriter) {
	stats, err := h.logger.GetStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *DashboardHandler) handleLogs(w http.ResponseWriter, r *http.Request) {
	limit := parseInt(r.URL.Query().Get("limit"), 100)
	offset := parseInt(r.URL.Query().Get("offset"), 0)
	if limit < 1 || limit > 500 {
		limit = 100
	}
	logs, err := h.logger.GetLogs(limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (h *DashboardHandler) handleLog(w http.ResponseWriter, uuid string) {
	log, err := h.logger.GetLogByUUID(uuid)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, log)
}

func (h *DashboardHandler) handleEvents(w http.ResponseWriter, uuid string) {
	events, err := h.logger.GetSSEEvents(uuid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func serveDashboardAsset(w http.ResponseWriter, r *http.Request, assetPath string) {
	if assetPath == "" || strings.Contains(assetPath, "..") || strings.HasPrefix(assetPath, "/") || strings.HasSuffix(assetPath, "/") {
		http.NotFound(w, r)
		return
	}

	data, err := dashboardStatic.ReadFile("dashboard_static/" + assetPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	switch {
	case strings.HasSuffix(assetPath, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(assetPath, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(assetPath, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	default:
		w.Header().Set("Content-Type", http.DetectContentType(data))
	}
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func parseInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
