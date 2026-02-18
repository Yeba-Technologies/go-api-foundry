//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/config"
	"github.com/Yeba-Technologies/go-api-foundry/config/router"
	"github.com/Yeba-Technologies/go-api-foundry/domain"
	"github.com/Yeba-Technologies/go-api-foundry/integration/testhelpers"
	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/Yeba-Technologies/go-api-foundry/internal/models"
	pkgredis "github.com/Yeba-Technologies/go-api-foundry/pkg/redis"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

// CacheIntegrationTestSuite tests health-check endpoints when a real Redis
// cache is available, ensuring the cache connectivity reported by the
// monitoring controller is accurate.
type CacheIntegrationTestSuite struct {
	suite.Suite
	db        *gorm.DB
	server    *httptest.Server
	baseURL   string
	logger    *log.Logger
	appConfig *config.ApplicationConfig
	cache     *pkgredis.RedisCache
}

func (suite *CacheIntegrationTestSuite) SetupSuite() {
	ctx := context.Background()

	// Start real Postgres
	db, _ := testhelpers.StartPostgres(ctx, suite.T())
	suite.db = db

	err := suite.db.AutoMigrate(&models.WaitlistEntry{})
	suite.Require().NoError(err)

	// Start real Redis
	redisC := testhelpers.StartRedis(ctx, suite.T())

	cache, err := pkgredis.NewRedisCache(&pkgredis.Config{
		Host: redisC.Host,
		Port: redisC.Port,
	})
	suite.Require().NoError(err, "should connect to test redis container")
	suite.cache = cache

	suite.logger = log.NewLoggerWithJSONOutput()

	suite.appConfig = &config.ApplicationConfig{
		DB:     suite.db,
		Logger: suite.logger,
		Cache:  cache,
	}

	suite.appConfig.RouterService = router.CreateRouterService(suite.logger, cache, &router.RouterConfig{
		RateLimitRequests: 100,
		RateLimitWindow:   time.Minute,
		RequestTimeout:    30 * time.Second,
	})

	domain.SetupCoreDomain(suite.appConfig)

	suite.server = httptest.NewServer(suite.appConfig.RouterService.GetEngine())
	suite.baseURL = suite.server.URL
}

func (suite *CacheIntegrationTestSuite) TearDownSuite() {
	if suite.server != nil {
		suite.server.Close()
	}
	if suite.cache != nil {
		err := suite.cache.Close()
		suite.Require().NoError(err, "Redis cache should close cleanly")
	}
	if suite.db != nil {
		sqlDB, _ := suite.db.DB()
		sqlDB.Close()
	}
}

// TestHealthCheckWithCache verifies that the health endpoint reports cache
// status = 1 when a real Redis instance is connected.
func (suite *CacheIntegrationTestSuite) TestHealthCheckWithCache() {
	resp, err := http.Get(suite.baseURL + "/health")
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.Require().NoError(err)

	data := response["data"].(map[string]interface{})
	suite.Equal(float64(1), data["database"], "database should be healthy")
	suite.Equal(float64(1), data["cache"], "cache should be healthy when Redis is up")
}

// TestCacheGetSetDelete exercises the core Redis Cache operations against a
// real Redis instance.
func (suite *CacheIntegrationTestSuite) TestCacheGetSetDelete() {
	ctx := context.Background()

	// SET
	err := suite.cache.Set(ctx, "integration-test-key", "hello-world", 10*time.Second)
	suite.Require().NoError(err)

	// GET
	val, err := suite.cache.Get(ctx, "integration-test-key")
	suite.Require().NoError(err)
	suite.Equal("hello-world", val)

	// DELETE
	err = suite.cache.Delete(ctx, "integration-test-key")
	suite.Require().NoError(err)

	// GET after DELETE should return empty
	val, err = suite.cache.Get(ctx, "integration-test-key")
	suite.Require().NoError(err)
	suite.Equal("", val)
}

// TestCachePing verifies Ping works against a live Redis.
func (suite *CacheIntegrationTestSuite) TestCachePing() {
	err := suite.cache.Ping(context.Background())
	suite.Require().NoError(err)
}

func TestCacheIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CacheIntegrationTestSuite))
}
