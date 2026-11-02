package inbound

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := NewMetrics(nil)
	r := gin.New()
	r.Use(m.Middleware())
	r.GET("/jobs/:id", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })
	r.GET("/metrics", m.Handler)
	for _, path := range []string{"/jobs/a", "/jobs/b", "/nope"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()
	for _, want := range []string{
		`http_requests_total{method="GET",route="/jobs/:id",status="200"} 2`, // the template, never the ids
		`http_requests_total{method="GET",route="unmatched",status="404"} 1`,
		`http_request_duration_seconds_bucket{route="/jobs/:id",le="+Inf"} 2`,
		`http_request_duration_seconds_count{route="/jobs/:id"} 2`,
		"go_goroutines ",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	if strings.Contains(body, "/jobs/a") || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain; version=0.0.4") {
		t.Errorf("raw path leaked or wrong content type: %s", w.Header().Get("Content-Type"))
	}
}
