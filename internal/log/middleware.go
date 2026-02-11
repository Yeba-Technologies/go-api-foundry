package log

import (
	"context"
	"net/http"
	"strings"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GenerateCorrelationID()
		ctx := context.WithValue(r.Context(), CorrelatedIDKey, id)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func LogRequest(l *Logger, r *http.Request) {
	l.WithCorrelationID(r.Context()).Info("HTTP request", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr, "user_agent", strings.Split(r.UserAgent(), "/")[0])
}
