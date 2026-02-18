package router

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

func metricsEnabled() bool {
	v := utils.GetEnvTrimmed("METRICS_ENABLED")
	if v == "" {
		return true
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return true
	}
	return b
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests.",
			},
			[]string{"method", "route", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "route", "status"},
		),
	}

	reg.MustRegister(m.requestsTotal, m.requestDuration)
	return m
}

func (routerService *RouterService) mountMetrics() {
	if !metricsEnabled() {
		routerService.logger.Info("Metrics disabled (METRICS_ENABLED=false)")
		return
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m := newMetrics(reg)

	// Middleware
	routerService.engine.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method

		m.requestsTotal.WithLabelValues(method, route, status).Inc()
		m.requestDuration.WithLabelValues(method, route, status).Observe(time.Since(start).Seconds())
	})

	// Endpoint
	h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	routerService.engine.GET("/metrics", gin.WrapH(h))

	// Avoid exposing metrics to cross-origin browser clients by default.
	routerService.engine.OPTIONS("/metrics", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNoContent)
	})

	routerService.logger.Info("Metrics endpoint mounted", "path", "/metrics")
}
