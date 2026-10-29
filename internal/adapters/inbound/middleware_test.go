package inbound

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newTestRouter(rpm, burst int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.SetTrustedProxies(nil)
	r.Use(RateLimit(rpm, burst), JSONBody(16, 32))
	r.POST("/x", func(ctx *gin.Context) {
		var body map[string]any
		if err := ctx.ShouldBindJSON(&body); err != nil {
			ctx.Status(http.StatusBadRequest)
			return
		}
		ctx.Status(http.StatusOK)
	})
	r.POST("/bodyless", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })
	return r
}

func do(r http.Handler, path, contentType, body, ip string) int {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if body == "" {
		req = httptest.NewRequest(http.MethodPost, path, nil)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.RemoteAddr = ip + ":1234"
	req.Header.Set("X-Forwarded-For", "9.9.9.9") // untrusted: must be ignored
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestJSONBody(t *testing.T) {
	r := newTestRouter(1000, 1000)
	for name, tc := range map[string]struct {
		path, contentType, body string
		want                    int
	}{
		"json ok":                {"/x", "application/json", `{"a":1}`, 200},
		"json with charset":      {"/x", "application/json; charset=utf-8", `{"a":1}`, 200},
		"form post rejected":     {"/x", "application/x-www-form-urlencoded", "a=1", 415},
		"text/plain rejected":    {"/x", "text/plain", `{"a":1}`, 415},
		"image upload passes":    {"/bodyless", "image/png", "\x89PNG", 200},
		"svg not an upload type": {"/bodyless", "image/svg+xml", "<svg/>", 415},
		"multipart rejected":     {"/bodyless", "multipart/form-data; boundary=x", "--x--", 415},
		"body without type":      {"/x", "", `{"a":1}`, 415},
		"empty form post":        {"/bodyless", "application/x-www-form-urlencoded", "", 415},
		"bodyless request":       {"/bodyless", "", "", 200},
		"oversized body is 400":  {"/x", "application/json", `{"a":"` + strings.Repeat("x", 64) + `"}`, 400},
	} {
		if got := do(r, tc.path, tc.contentType, tc.body, "10.0.0.1"); got != tc.want {
			t.Errorf("%s: got %d, want %d", name, got, tc.want)
		}
	}
}

func TestRateLimit(t *testing.T) {
	r := newTestRouter(60, 3)
	for i := range 3 {
		if got := do(r, "/bodyless", "", "", "10.0.0.2"); got != 200 {
			t.Fatalf("request %d within burst: got %d", i, got)
		}
	}
	if got := do(r, "/bodyless", "", "", "10.0.0.2"); got != 429 {
		t.Fatalf("over burst: got %d, want 429", got)
	}
	// Another client has its own bucket, even with the same spoofed X-Forwarded-For.
	if got := do(r, "/bodyless", "", "", "10.0.0.3"); got != 200 {
		t.Fatalf("other client: got %d, want 200", got)
	}
}

func TestRequestTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.ContextWithFallback = true // as in router.go
	r.Use(RequestTimeout(20 * time.Millisecond))
	r.GET("/slow", func(ctx *gin.Context) {
		// Handlers pass *gin.Context straight to the DB, Redis and IAM calls.
		if _, ok := ctx.Deadline(); !ok {
			ctx.Status(http.StatusInternalServerError)
			return
		}
		select {
		case <-ctx.Done():
			ctx.Status(http.StatusGatewayTimeout)
		case <-time.After(time.Second):
			ctx.Status(http.StatusOK)
		}
	})
	rec := httptest.NewRecorder()
	start := time.Now()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slow", nil))
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("got %d, want the handler's context cancelled (504)", rec.Code)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("handler ran %v; the deadline did not cancel it", elapsed)
	}
}
