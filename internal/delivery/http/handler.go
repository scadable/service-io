// Minimal, JSON-only REST API.
package http

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
		h.addDevice(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/devices":
		h.listDevices(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) addDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Type == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	dev, err := h.mgr.AddDevice(ctx, req.Type)
	if err != nil {
		h.lg.Error().Err(err).Msg("add device")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dev)
}

func (h *Handler) listDevices(w http.ResponseWriter, _ *http.Request) {
	devs, err := h.mgr.ListDevices()
	if err != nil {
		h.lg.Error().Err(err).Msg("list devices")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(devs)
}
