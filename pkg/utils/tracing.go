package utils

import (
	"os"
	"strconv"
	"strings"
)

func IsTracingEnabled() bool {
	v := strings.TrimSpace(os.Getenv("OTEL_TRACES_ENABLED"))

	if v == "" {
		return false
	}

	b, err := strconv.ParseBool(v)

	if err != nil {
		return false
	}

	return b
}

func OTelServiceName() string {
	serviceName := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if serviceName == "" {
		serviceName = "go-api-foundry"
	}
	return serviceName
}
