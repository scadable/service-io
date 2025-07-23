// Simple JSON REST surface: POST /devices  ,  GET /devices
package api

import (
	"encoding/json"
	"net/http"

	"service-io/internal/core/devices"

	"github.com/rs/zerolog"
)

type Handler struct {
	mgr *devices.Manager
	lg  zerolog.Logger
}

func New(m *devices.Manager, lg zerolog.Logger) *Handler {
	return &Handler{mgr: m, lg: lg}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/devices":
		h.handleAdd(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/devices":
		h.handleList(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Type == "" {
		http.Error(w, "body must be {\"type\":\"<deviceType>\"}", http.StatusBadRequest)
		return
	}
	dev, err := h.mgr.AddDevice(r.Context(), req.Type)
	if err != nil {
		h.lg.Error().Err(err).Msg("add device")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, dev)
}

func (h *Handler) handleList(w http.ResponseWriter, _ *http.Request) {
	list, err := h.mgr.ListDevices()
	if err != nil {
		h.lg.Error().Err(err).Msg("list devices")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
