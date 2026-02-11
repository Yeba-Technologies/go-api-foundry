package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/config"
	"github.com/Yeba-Technologies/go-api-foundry/config/router"
	"github.com/Yeba-Technologies/go-api-foundry/domain"
	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/Yeba-Technologies/go-api-foundry/internal/models"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type WaitlistAPITestSuite struct {
	suite.Suite
	db        *gorm.DB
	server    *httptest.Server
	baseURL   string
	logger    *log.Logger
	appConfig *config.ApplicationConfig
}

func (suite *WaitlistAPITestSuite) SetupSuite() {
	var err error
	suite.db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	suite.Require().NoError(err)

	err = suite.db.AutoMigrate(&models.WaitlistEntry{})
	suite.Require().NoError(err)

	suite.logger = log.NewLoggerWithJSONOutput()

	suite.appConfig = &config.ApplicationConfig{
		DB:     suite.db,
		Logger: suite.logger,
	}

	suite.appConfig.RouterService = router.CreateRouterService(suite.logger, nil, &router.RouterConfig{
		RateLimitRequests: 100,
		RateLimitWindow:   time.Minute,
		RequestTimeout:    30 * time.Second,
	})

	domain.SetupCoreDomain(suite.appConfig)

	suite.server = httptest.NewServer(suite.appConfig.RouterService.GetEngine())
	suite.baseURL = suite.server.URL
}

func (suite *WaitlistAPITestSuite) TearDownSuite() {
	if suite.server != nil {
		suite.server.Close()
	}
	if suite.db != nil {
		sqlDB, _ := suite.db.DB()
		sqlDB.Close()
	}
}

func (suite *WaitlistAPITestSuite) SetupTest() {
	suite.db.Exec("DELETE FROM waitlist_entries")
}

func (suite *WaitlistAPITestSuite) TestHealthCheck() {
	resp, err := http.Get(suite.baseURL + "/health")
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.Require().NoError(err)

	suite.Equal(float64(200), response["code"])
	suite.Contains(response["message"], "health check completed")

	data := response["data"].(map[string]interface{})
	suite.Contains(data, "database")
	suite.Contains(data, "uptime")

	suite.Equal(float64(1), data["database"])
}

func (suite *WaitlistAPITestSuite) TestCreateWaitlistEntry() {
	requestBody := map[string]string{
		"email":      "john.doe@example.com",
		"first_name": "John",
		"last_name":  "Doe",
	}

	jsonBody, _ := json.Marshal(requestBody)

	resp, err := http.Post(suite.baseURL+"/v1/waitlist", "application/json", bytes.NewBuffer(jsonBody))
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusCreated, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.Require().NoError(err)

	suite.Equal(float64(201), response["code"])
	suite.Contains(response["message"], "created successfully")

	data := response["data"].(map[string]interface{})
	suite.Equal("john.doe@example.com", data["email"])
	suite.Equal("John", data["first_name"])
	suite.Equal("Doe", data["last_name"])
	suite.Contains(data, "id")
	suite.Contains(data, "created_at")
}

func (suite *WaitlistAPITestSuite) TestCreateWaitlistEntryValidationError() {
	requestBody := map[string]string{
		"email":      "invalid-email",
		"first_name": "",
		"last_name":  "Doe",
	}

	jsonBody, _ := json.Marshal(requestBody)

	resp, err := http.Post(suite.baseURL+"/v1/waitlist", "application/json", bytes.NewBuffer(jsonBody))
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.Require().NoError(err)

	suite.Equal(float64(400), response["code"])
	suite.Contains(response["message"], "Invalid request payload")

	data := response["data"].([]interface{})
	suite.True(len(data) > 0)

	// Check that validation errors are present
	foundEmailError := false
	foundFirstNameError := false
	for _, item := range data {
		fieldError := item.(map[string]interface{})
		field := fieldError["field"].(string)
		message := fieldError["message"].(string)

		if field == "email" {
			foundEmailError = true
			suite.Contains(message, "Invalid email format")
		}
		if field == "first_name" {
			foundFirstNameError = true
			suite.Contains(message, "required")
		}
	}

	suite.True(foundEmailError, "Should have email validation error")
	suite.True(foundFirstNameError, "Should have first_name validation error")
}

func (suite *WaitlistAPITestSuite) TestGetAllWaitlistEntries() {
	// First create some test data
	entries := []models.WaitlistEntry{
		{Email: "user1@example.com", FirstName: "User", LastName: "One"},
		{Email: "user2@example.com", FirstName: "User", LastName: "Two"},
	}

	for _, entry := range entries {
		err := suite.db.Create(&entry).Error
		suite.Require().NoError(err)
	}

	// Now test the GET endpoint
	resp, err := http.Get(suite.baseURL + "/v1/waitlist")
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.Require().NoError(err)

	suite.Equal(float64(200), response["code"])
	suite.Contains(response["message"], "retrieved successfully")

	data := response["data"].([]interface{})
	suite.Len(data, 2)

	// Check that the entries are returned
	emails := make([]string, len(data))
	for i, item := range data {
		entry := item.(map[string]interface{})
		emails[i] = entry["email"].(string)
	}

	suite.Contains(emails, "user1@example.com")
	suite.Contains(emails, "user2@example.com")
}

func (suite *WaitlistAPITestSuite) TestGetWaitlistEntryByID() {
	// Create a test entry
	entry := models.WaitlistEntry{
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
	}
	err := suite.db.Create(&entry).Error
	suite.Require().NoError(err)

	// Test GET by ID
	url := fmt.Sprintf("%s/v1/waitlist/%d", suite.baseURL, entry.ID)
	resp, err := http.Get(url)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.Require().NoError(err)

	suite.Equal(float64(200), response["code"])
	data := response["data"].(map[string]interface{})
	suite.Equal("test@example.com", data["email"])
	suite.Equal("Test", data["first_name"])
	suite.Equal("User", data["last_name"])
}

func (suite *WaitlistAPITestSuite) TestGetWaitlistEntryByIDNotFound() {
	// Test GET by ID for non-existent entry
	resp, err := http.Get(suite.baseURL + "/v1/waitlist/999")
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.Require().NoError(err)

	suite.Equal(float64(404), response["code"])
	suite.Contains(response["message"], "not found")
}

func (suite *WaitlistAPITestSuite) TestUpdateWaitlistEntry() {
	// Create a test entry
	entry := models.WaitlistEntry{
		Email:     "original@example.com",
		FirstName: "Original",
		LastName:  "User",
	}
	err := suite.db.Create(&entry).Error
	suite.Require().NoError(err)

	// Update the entry
	requestBody := map[string]string{
		"email":      "updated@example.com",
		"first_name": "Updated",
	}

	jsonBody, _ := json.Marshal(requestBody)

	url := fmt.Sprintf("%s/v1/waitlist/%d", suite.baseURL, entry.ID)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonBody))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.Require().NoError(err)

	suite.Equal(float64(200), response["code"])
	suite.Contains(response["message"], "updated successfully")

	// Verify the update in database
	var updatedEntry models.WaitlistEntry
	err = suite.db.First(&updatedEntry, entry.ID).Error
	suite.Require().NoError(err)
	suite.Equal("updated@example.com", updatedEntry.Email)
	suite.Equal("Updated", updatedEntry.FirstName)
	suite.Equal("User", updatedEntry.LastName) // Should remain unchanged
}

func (suite *WaitlistAPITestSuite) TestDeleteWaitlistEntry() {
	// Create a test entry
	entry := models.WaitlistEntry{
		Email:     "delete@example.com",
		FirstName: "Delete",
		LastName:  "Me",
	}
	err := suite.db.Create(&entry).Error
	suite.Require().NoError(err)

	// Delete the entry
	url := fmt.Sprintf("%s/v1/waitlist/%d", suite.baseURL, entry.ID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	suite.Require().NoError(err)

	client := &http.Client{}
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.Require().NoError(err)

	suite.Equal(float64(200), response["code"])
	suite.Contains(response["message"], "deleted successfully")

	// Verify the entry is deleted
	var deletedEntry models.WaitlistEntry
	err = suite.db.First(&deletedEntry, entry.ID).Error
	suite.True(err != nil) // Should return error because entry is deleted
}

func (suite *WaitlistAPITestSuite) TestDuplicateEmailError() {
	// Create first entry
	entry1 := models.WaitlistEntry{
		Email:     "duplicate@example.com",
		FirstName: "First",
		LastName:  "User",
	}
	err := suite.db.Create(&entry1).Error
	suite.Require().NoError(err)

	// Try to create second entry with same email
	requestBody := map[string]string{
		"email":      "duplicate@example.com",
		"first_name": "Second",
		"last_name":  "User",
	}

	jsonBody, _ := json.Marshal(requestBody)

	resp, err := http.Post(suite.baseURL+"/v1/waitlist", "application/json", bytes.NewBuffer(jsonBody))
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusConflict, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.Require().NoError(err)

	suite.Equal(float64(409), response["code"])
	suite.Contains(response["message"], "already exists")
}

func TestSimpleHealthCheck(t *testing.T) {
	// Skip integration tests unless explicitly requested
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration tests. Set RUN_INTEGRATION_TESTS=true to run them")
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	err = db.AutoMigrate(&models.WaitlistEntry{})
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	logger := log.NewLoggerWithJSONOutput()

	appConfig := &config.ApplicationConfig{
		DB:     db,
		Logger: logger,
	}

	appConfig.RouterService = router.CreateRouterService(logger, nil, &router.RouterConfig{
		RateLimitRequests: 100,
		RateLimitWindow:   time.Minute,
		RequestTimeout:    30 * time.Second,
	})

	domain.SetupCoreDomain(appConfig)

	server := httptest.NewServer(appConfig.RouterService.GetEngine())
	defer server.Close()
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["code"].(float64) != 200 {
		t.Errorf("Expected code 200, got %v", response["code"])
	}

	t.Logf("Health check response: %+v", response)
}

func TestWaitlistAPISuite(t *testing.T) {
	// Skip integration tests unless explicitly requested
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration tests. Set RUN_INTEGRATION_TESTS=true to run them")
	}

	suite.Run(t, new(WaitlistAPITestSuite))
}
