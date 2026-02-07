package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/org/nexus/internal/hash"
	"github.com/org/nexus/internal/models"
	"github.com/org/nexus/internal/store"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	store     store.SnapshotStore
	hasher    hash.Generator
	validator *validator.Validate
	logger    *zap.Logger
}

// NewHandler creates a new Handler.
func NewHandler(s store.SnapshotStore, logger *zap.Logger) *Handler {
	return &Handler{
		store:     s,
		hasher:    hash.NewGenerator(),
		validator: validator.New(),
		logger:    logger,
	}
}

// LogSnapshot handles POST /api/v1/log.
func (h *Handler) LogSnapshot(w http.ResponseWriter, r *http.Request) {
	var req models.LogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	if err := h.validator.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	now := time.Now().UTC()
	req.Request.Timestamp = now
	req.Response.Timestamp = now

	hashStr, err := h.hasher.Generate(req.Request, req.Response, req.AttributeState)
	if err != nil {
		h.logger.Error("failed to generate hash", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to generate hash")
		return
	}

	snapshot := &models.Snapshot{
		Hash:           hashStr,
		Request:        req.Request,
		Response:       req.Response,
		AttributeState: req.AttributeState,
	}

	result, err := h.store.Upsert(r.Context(), snapshot)
	if err != nil {
		h.logger.Error("failed to upsert snapshot", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to store snapshot")
		return
	}

	writeJSON(w, http.StatusOK, models.LogResponse{
		Hash:            hashStr,
		IsNew:           result.IsNew,
		OccurrenceCount: result.OccurrenceCount,
	})
}

// ListSnapshots handles GET /api/v1/snapshots.
func (h *Handler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	query := models.SnapshotQuery{
		Path:   r.URL.Query().Get("path"),
		Method: r.URL.Query().Get("method"),
		From:   r.URL.Query().Get("from"),
		To:     r.URL.Query().Get("to"),
	}

	if sc := r.URL.Query().Get("statusCode"); sc != "" {
		code, err := strconv.Atoi(sc)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid statusCode parameter")
			return
		}
		query.StatusCode = code
	}

	if l := r.URL.Query().Get("limit"); l != "" {
		limit, err := strconv.ParseInt(l, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid limit parameter")
			return
		}
		query.Limit = limit
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		offset, err := strconv.ParseInt(o, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid offset parameter")
			return
		}
		query.Offset = offset
	}

	snapshots, total, err := h.store.Find(r.Context(), query)
	if err != nil {
		h.logger.Error("failed to find snapshots", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to query snapshots")
		return
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	writeJSON(w, http.StatusOK, models.SnapshotListResponse{
		Data:   snapshots,
		Total:  total,
		Limit:  limit,
		Offset: query.Offset,
	})
}

// GetSnapshot handles GET /api/v1/snapshots/{hash}.
func (h *Handler) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	hashParam := chi.URLParam(r, "hash")
	if hashParam == "" {
		writeError(w, http.StatusBadRequest, "hash parameter is required")
		return
	}

	snapshot, err := h.store.FindByHash(r.Context(), hashParam)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "snapshot not found")
			return
		}
		h.logger.Error("failed to find snapshot", zap.Error(err), zap.String("hash", hashParam))
		writeError(w, http.StatusInternalServerError, "failed to retrieve snapshot")
		return
	}

	writeJSON(w, http.StatusOK, snapshot)
}

// HealthCheck handles GET /health.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	err := h.store.HealthCheck(r.Context())
	status := "healthy"
	httpStatus := http.StatusOK

	if err != nil {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
		h.logger.Warn("health check failed", zap.Error(err))
	}

	writeJSON(w, httpStatus, map[string]interface{}{
		"status":  status,
		"version": "1.0.0",
		"dependencies": map[string]string{
			"mongodb": status,
		},
	})
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}

func writeValidationError(w http.ResponseWriter, err error) {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := make([]map[string]string, 0, len(ve))
		for _, fe := range ve {
			fields = append(fields, map[string]string{
				"field":   fe.Field(),
				"tag":     fe.Tag(),
				"message": fe.Error(),
			})
		}
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "Validation Failed",
			"fields": fields,
		})
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}
