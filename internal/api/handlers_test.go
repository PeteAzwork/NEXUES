package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/org/nexus/internal/models"
	"github.com/org/nexus/internal/store"
)

func newTestRouter() (*store.MockStore, http.Handler) {
	mockStore := store.NewMockStore()
	logger := zap.NewNop()
	router := NewRouter(mockStore, logger)
	return mockStore, router
}

func TestLogSnapshot_Success(t *testing.T) {
	_, router := newTestRouter()

	payload := models.LogRequest{
		Request: models.Request{
			Method: "GET",
			Path:   "/api/test",
		},
		Response: models.Response{
			StatusCode: 200,
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.LogResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.Hash)
	assert.True(t, resp.IsNew)
	assert.Equal(t, int64(1), resp.OccurrenceCount)
}

func TestLogSnapshot_DuplicateIncrements(t *testing.T) {
	_, router := newTestRouter()

	payload := models.LogRequest{
		Request: models.Request{
			Method: "POST",
			Path:   "/api/data",
		},
		Response: models.Response{
			StatusCode: 201,
		},
	}

	body, _ := json.Marshal(payload)

	// First request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp1 models.LogResponse
	json.NewDecoder(w.Body).Decode(&resp1)
	assert.True(t, resp1.IsNew)

	// Second request (same data)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp2 models.LogResponse
	json.NewDecoder(w.Body).Decode(&resp2)
	assert.False(t, resp2.IsNew)
	assert.Equal(t, int64(2), resp2.OccurrenceCount)
	assert.Equal(t, resp1.Hash, resp2.Hash)
}

func TestLogSnapshot_InvalidJSON(t *testing.T) {
	_, router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/log", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogSnapshot_ValidationError(t *testing.T) {
	_, router := newTestRouter()

	// Missing required fields
	payload := models.LogRequest{
		Request: models.Request{
			Method: "INVALID",
			Path:   "/api/test",
		},
		Response: models.Response{
			StatusCode: 200,
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListSnapshots_Empty(t *testing.T) {
	_, router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.SnapshotListResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Empty(t, resp.Data)
	assert.Equal(t, int64(0), resp.Total)
}

func TestListSnapshots_WithData(t *testing.T) {
	_, router := newTestRouter()

	// Add a snapshot first
	payload := models.LogRequest{
		Request:  models.Request{Method: "GET", Path: "/api/users"},
		Response: models.Response{StatusCode: 200},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Now list
	req = httptest.NewRequest(http.MethodGet, "/api/v1/snapshots", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.SnapshotListResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, int64(1), resp.Total)
	assert.Len(t, resp.Data, 1)
}

func TestListSnapshots_InvalidStatusCode(t *testing.T) {
	_, router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots?statusCode=abc", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetSnapshot_Found(t *testing.T) {
	_, router := newTestRouter()

	// Add a snapshot
	payload := models.LogRequest{
		Request:  models.Request{Method: "GET", Path: "/api/found"},
		Response: models.Response{StatusCode: 200},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var logResp models.LogResponse
	json.NewDecoder(w.Body).Decode(&logResp)

	// Fetch it by hash
	req = httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/"+logResp.Hash, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var snapshot models.Snapshot
	err := json.NewDecoder(w.Body).Decode(&snapshot)
	require.NoError(t, err)
	assert.Equal(t, logResp.Hash, snapshot.Hash)
}

func TestGetSnapshot_NotFound(t *testing.T) {
	_, router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/nonexistenthash", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHealthCheck_Healthy(t *testing.T) {
	_, router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, "healthy", resp["status"])
}

func TestHealthCheck_Unhealthy(t *testing.T) {
	mockStore, router := newTestRouter()
	mockStore.SetHealthy(false)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, "unhealthy", resp["status"])
}
