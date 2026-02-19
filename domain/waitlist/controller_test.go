package waitlist

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Yeba-Technologies/go-api-foundry/config/router"
	apperrors "github.com/Yeba-Technologies/go-api-foundry/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// jsonResponse is the envelope every handler response is wrapped in.
type jsonResponse struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

// setupTestRouter creates a gin engine in test mode with the custom trim
// validator registered, matching production behaviour.
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("trim", func(fl validator.FieldLevel) bool {
			if fl.Field().Kind() != reflect.String {
				return true
			}
			return fl.Field().String() == strings.TrimSpace(fl.Field().String())
		})
	}
	return engine
}

// wrapTestHandler converts a router.HandlerFunction into a gin.HandlerFunc,
// replicating the wrapping logic from config/router/controller.go.
func wrapTestHandler(h router.HandlerFunction) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := h(c)
		if result == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"data":    nil,
				"message": "handler returned nil result",
			})
			return
		}
		c.JSON(result.StatusCode, result.ToJSON())
	}
}

// parseResponse decodes the JSON body into our standard envelope.
func parseResponse(t *testing.T, w *httptest.ResponseRecorder) jsonResponse {
	t.Helper()
	var resp jsonResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	return resp
}

// jsonBody marshals v to a *bytes.Reader for use as a request body.
func jsonBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(b)
}

func TestCreateHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	expected := &WaitlistEntryResponse{
		ID: 1, Email: "alice@example.com", FirstName: "Alice", LastName: "Smith", CreatedAt: "2025-01-01T00:00:00Z",
	}
	svc.EXPECT().CreateEntry(gomock.Any(), gomock.Any()).Return(expected, nil)

	r := setupTestRouter()
	r.POST("/v1/waitlist", wrapTestHandler(createWaitlistEntryHandler(svc)))

	body := jsonBody(t, map[string]string{
		"email": "alice@example.com", "first_name": "Alice", "last_name": "Smith",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/waitlist", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, http.StatusCreated, resp.Code)
	assert.Contains(t, resp.Message, "created successfully")

	var data WaitlistEntryResponse
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	assert.Equal(t, "alice@example.com", data.Email)
	assert.Equal(t, uint(1), data.ID)
}

func TestCreateHandler_MissingRequiredFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	r := setupTestRouter()
	r.POST("/v1/waitlist", wrapTestHandler(createWaitlistEntryHandler(svc)))

	body := jsonBody(t, map[string]string{"email": "alice@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/v1/waitlist", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "Invalid request payload", resp.Message)

	var validationErrors []apperrors.ValidationErrorResponse
	require.NoError(t, json.Unmarshal(resp.Data, &validationErrors))
	assert.GreaterOrEqual(t, len(validationErrors), 1, "should report at least one validation error")

	fields := make(map[string]bool)
	for _, ve := range validationErrors {
		fields[ve.Field] = true
	}
	assert.True(t, fields["first_name"], "first_name should be flagged")
	assert.True(t, fields["last_name"], "last_name should be flagged")
}

func TestCreateHandler_InvalidEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	r := setupTestRouter()
	r.POST("/v1/waitlist", wrapTestHandler(createWaitlistEntryHandler(svc)))

	body := jsonBody(t, map[string]string{
		"email": "not-an-email", "first_name": "Alice", "last_name": "Smith",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/waitlist", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	resp := parseResponse(t, w)

	var validationErrors []apperrors.ValidationErrorResponse
	require.NoError(t, json.Unmarshal(resp.Data, &validationErrors))
	assert.GreaterOrEqual(t, len(validationErrors), 1)

	found := false
	for _, ve := range validationErrors {
		if ve.Field == "email" {
			found = true
			assert.Contains(t, strings.ToLower(ve.Message), "email")
		}
	}
	assert.True(t, found, "email field should have a validation error")
}

func TestCreateHandler_MalformedJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	r := setupTestRouter()
	r.POST("/v1/waitlist", wrapTestHandler(createWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodPost, "/v1/waitlist", strings.NewReader(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	resp := parseResponse(t, w)
	assert.Contains(t, resp.Message, "Invalid request")
}

func TestCreateHandler_EmptyBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	r := setupTestRouter()
	r.POST("/v1/waitlist", wrapTestHandler(createWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodPost, "/v1/waitlist", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateHandler_ServiceConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	svc.EXPECT().CreateEntry(gomock.Any(), gomock.Any()).
		Return(nil, apperrors.NewConflictError("email already exists", nil))

	r := setupTestRouter()
	r.POST("/v1/waitlist", wrapTestHandler(createWaitlistEntryHandler(svc)))

	body := jsonBody(t, map[string]string{
		"email": "dup@example.com", "first_name": "Bob", "last_name": "Jones",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/waitlist", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, http.StatusConflict, resp.Code)
	assert.Equal(t, "email already exists", resp.Message)
}

func TestCreateHandler_ServiceDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	svc.EXPECT().CreateEntry(gomock.Any(), gomock.Any()).
		Return(nil, apperrors.NewDatabaseError("connection lost", nil))

	r := setupTestRouter()
	r.POST("/v1/waitlist", wrapTestHandler(createWaitlistEntryHandler(svc)))

	body := jsonBody(t, map[string]string{
		"email": "db@example.com", "first_name": "Eve", "last_name": "Doe",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/waitlist", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestCreateHandler_FieldExceedsMaxLength(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	r := setupTestRouter()
	r.POST("/v1/waitlist", wrapTestHandler(createWaitlistEntryHandler(svc)))

	longName := strings.Repeat("a", 256)
	body := jsonBody(t, map[string]string{
		"email": "max@example.com", "first_name": longName, "last_name": "Smith",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/waitlist", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	resp := parseResponse(t, w)

	var validationErrors []apperrors.ValidationErrorResponse
	require.NoError(t, json.Unmarshal(resp.Data, &validationErrors))
	found := false
	for _, ve := range validationErrors {
		if ve.Field == "first_name" {
			found = true
		}
	}
	assert.True(t, found, "first_name should trigger max-length validation")
}

func TestGetHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	expected := &WaitlistEntryResponse{
		ID: 42, Email: "alice@example.com", FirstName: "Alice", LastName: "Smith", CreatedAt: "2025-01-01T00:00:00Z",
	}
	svc.EXPECT().FindEntryByID(gomock.Any(), uint(42)).Return(expected, nil)

	r := setupTestRouter()
	r.GET("/v1/waitlist/:id", wrapTestHandler(getWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodGet, "/v1/waitlist/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, http.StatusOK, resp.Code)

	var data WaitlistEntryResponse
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	assert.Equal(t, uint(42), data.ID)
	assert.Equal(t, "alice@example.com", data.Email)
}

func TestGetHandler_InvalidID(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	r := setupTestRouter()
	r.GET("/v1/waitlist/:id", wrapTestHandler(getWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodGet, "/v1/waitlist/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	resp := parseResponse(t, w)
	assert.Contains(t, resp.Message, "Invalid ID")
}

func TestGetHandler_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	svc.EXPECT().FindEntryByID(gomock.Any(), uint(999)).
		Return(nil, apperrors.NewNotFoundError("entry not found", nil))

	r := setupTestRouter()
	r.GET("/v1/waitlist/:id", wrapTestHandler(getWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodGet, "/v1/waitlist/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, "entry not found", resp.Message)
}

func TestGetHandler_ServiceDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	svc.EXPECT().FindEntryByID(gomock.Any(), uint(1)).
		Return(nil, apperrors.NewDatabaseError("timeout", nil))

	r := setupTestRouter()
	r.GET("/v1/waitlist/:id", wrapTestHandler(getWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodGet, "/v1/waitlist/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUpdateHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	svc.EXPECT().UpdateEntry(gomock.Any(), uint(1), gomock.Any()).Return(nil)

	r := setupTestRouter()
	r.PUT("/v1/waitlist/:id", wrapTestHandler(updateWaitlistEntryHandler(svc)))

	body := jsonBody(t, map[string]string{"first_name": "Updated"})
	req := httptest.NewRequest(http.MethodPut, "/v1/waitlist/1", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Contains(t, resp.Message, "updated successfully")
}

func TestUpdateHandler_InvalidID(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	r := setupTestRouter()
	r.PUT("/v1/waitlist/:id", wrapTestHandler(updateWaitlistEntryHandler(svc)))

	body := jsonBody(t, map[string]string{"first_name": "Updated"})
	req := httptest.NewRequest(http.MethodPut, "/v1/waitlist/xyz", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateHandler_InvalidEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	r := setupTestRouter()
	r.PUT("/v1/waitlist/:id", wrapTestHandler(updateWaitlistEntryHandler(svc)))

	body := jsonBody(t, map[string]string{"email": "bad-email"})
	req := httptest.NewRequest(http.MethodPut, "/v1/waitlist/1", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateHandler_MalformedJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	r := setupTestRouter()
	r.PUT("/v1/waitlist/:id", wrapTestHandler(updateWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodPut, "/v1/waitlist/1", strings.NewReader(`{not json}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateHandler_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	svc.EXPECT().UpdateEntry(gomock.Any(), uint(99), gomock.Any()).
		Return(apperrors.NewNotFoundError("entry not found", nil))

	r := setupTestRouter()
	r.PUT("/v1/waitlist/:id", wrapTestHandler(updateWaitlistEntryHandler(svc)))

	body := jsonBody(t, map[string]string{"first_name": "Nope"})
	req := httptest.NewRequest(http.MethodPut, "/v1/waitlist/99", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, "entry not found", resp.Message)
}

func TestGetAllHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	entries := []WaitlistEntryResponse{
		{ID: 1, Email: "a@b.com", FirstName: "A", LastName: "B", CreatedAt: "2025-01-01T00:00:00Z"},
		{ID: 2, Email: "c@d.com", FirstName: "C", LastName: "D", CreatedAt: "2025-01-02T00:00:00Z"},
	}
	svc.EXPECT().GetAllEntries(gomock.Any()).Return(entries, nil)

	r := setupTestRouter()
	r.GET("/v1/waitlist", wrapTestHandler(getAllWaitlistEntriesHandler(svc)))

	req := httptest.NewRequest(http.MethodGet, "/v1/waitlist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)

	var data []WaitlistEntryResponse
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	assert.Len(t, data, 2)
}

func TestGetAllHandler_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	svc.EXPECT().GetAllEntries(gomock.Any()).Return([]WaitlistEntryResponse{}, nil)

	r := setupTestRouter()
	r.GET("/v1/waitlist", wrapTestHandler(getAllWaitlistEntriesHandler(svc)))

	req := httptest.NewRequest(http.MethodGet, "/v1/waitlist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)

	var data []WaitlistEntryResponse
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	assert.Empty(t, data)
}

func TestGetAllHandler_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	svc.EXPECT().GetAllEntries(gomock.Any()).
		Return(nil, apperrors.NewDatabaseError("connection refused", nil))

	r := setupTestRouter()
	r.GET("/v1/waitlist", wrapTestHandler(getAllWaitlistEntriesHandler(svc)))

	req := httptest.NewRequest(http.MethodGet, "/v1/waitlist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeleteHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	svc.EXPECT().DeleteEntry(gomock.Any(), uint(5)).Return(nil)

	r := setupTestRouter()
	r.DELETE("/v1/waitlist/:id", wrapTestHandler(deleteWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodDelete, "/v1/waitlist/5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(t, w)
	assert.Contains(t, resp.Message, "deleted successfully")
}

func TestDeleteHandler_InvalidID(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	r := setupTestRouter()
	r.DELETE("/v1/waitlist/:id", wrapTestHandler(deleteWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodDelete, "/v1/waitlist/notanumber", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteHandler_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	svc.EXPECT().DeleteEntry(gomock.Any(), uint(404)).
		Return(apperrors.NewNotFoundError("entry not found", nil))

	r := setupTestRouter()
	r.DELETE("/v1/waitlist/:id", wrapTestHandler(deleteWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodDelete, "/v1/waitlist/404", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	resp := parseResponse(t, w)
	assert.Equal(t, "entry not found", resp.Message)
}

func TestDeleteHandler_ServiceDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	svc.EXPECT().DeleteEntry(gomock.Any(), uint(1)).
		Return(apperrors.NewDatabaseError("disk full", nil))

	r := setupTestRouter()
	r.DELETE("/v1/waitlist/:id", wrapTestHandler(deleteWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodDelete, "/v1/waitlist/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateHandler_WrongContentType(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	r := setupTestRouter()
	r.POST("/v1/waitlist", wrapTestHandler(createWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodPost, "/v1/waitlist", strings.NewReader("email=a@b.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateHandler_TypeMismatchInJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMockWaitlistService(ctrl)

	r := setupTestRouter()
	r.POST("/v1/waitlist", wrapTestHandler(createWaitlistEntryHandler(svc)))

	req := httptest.NewRequest(http.MethodPost, "/v1/waitlist",
		strings.NewReader(`{"email": 12345, "first_name": "A", "last_name": "B"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
