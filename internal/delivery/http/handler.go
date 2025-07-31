// Simple JSON REST surface: POST /devices  ,  GET /devices
package api

import (
	"encoding/json"
	"net/http"
	"service-io/internal/core/devices"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	httpSwagger "github.com/swaggo/http-swagger"
)

type Handler struct {
	mgr *devices.Manager
	lg  zerolog.Logger
}

// addDeviceRequest defines the shape of the request body for adding a device.
type addDeviceRequest struct {
	Type string `json:"type" example:"random"`
}

func New(m *devices.Manager, lg zerolog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	h := &Handler{mgr: m, lg: lg}

	// --- API Routes ---
	r.Route("/devices", func(r chi.Router) {
		r.Post("/", h.handleAdd)
		r.Get("/", h.handleList)
	})

	// --- Swagger Docs Route ---
	// This new route redirects from /docs to /docs/index.html
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/index.html", http.StatusMovedPermanently)
	})
	r.Get("/docs/*", httpSwagger.WrapHandler)

	return r
}

// handleAdd creates a new device adapter.
// @Summary      Add a new device
// @Description  Creates a new device adapter instance, runs its container, and returns the device details.
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        device  body      addDeviceRequest     true  "Device Type"
// @Success      200     {object}  devices.Device
// @Failure      400     {string}  string "Bad Request"
// @Failure      500     {string}  string "Internal Server Error"
// @Router       /devices [post]
func (h *Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	var req addDeviceRequest
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

// handleList lists all registered device adapters.
// @Summary      List all devices
// @Description  Retrieves a list of all device adapters from the database.
// @Tags         devices
// @Produce      json
// @Success      200  {array}   devices.Device
// @Failure      500  {string}  string "Internal Server Error"
// @Router       /devices [get]
func (h *Handler) handleList(w http.ResponseWriter, _ *http.Request) {
	list, err := h.mgr.ListDevices()
	if err != nil {
		h.lg.Error().Err(err).Msg("list devices")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

// handleDelete removes a device adapter.
// @Summary      Remove a device
// @Description  Stops the device's container and removes its record from the database.
// @Tags         devices
// @Produce      json
// @Param        deviceID   path      string  true  "Device ID"
// @Success      204  {string}  string "No Content"
// @Failure      404  {string}  string "Not Found"
// @Failure      500  {string}  string "Internal Server Error"
// @Router       /devices/{deviceID} [delete]
func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")
	if err := h.mgr.RemoveDevice(r.Context(), deviceID); err != nil {
		// A bit of logic to return a 404 for "not found" errors
		if err.Error() == "device with ID '"+deviceID+"' not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
