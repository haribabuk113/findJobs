package inbound

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuthHandlerIsSecure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name           string
		secureCookie   bool
		trustedProxies []string
		remoteAddr     string
		xfp            string // X-Forwarded-Proto header
		useTLS         bool
		want           bool
	}{
		{
			name:         "direct TLS connection is secure",
			secureCookie: false,
			useTLS:       true,
			want:         true,
		},
		{
			name:         "plain HTTP dev falls back to configured false",
			secureCookie: false,
			want:         false,
		},
		{
			name:         "plain HTTP prod falls back to configured true",
			secureCookie: true,
			want:         true,
		},
		{
			name:           "trusted proxy says https",
			secureCookie:   false,
			trustedProxies: []string{"10.0.0.0/8"},
			remoteAddr:     "10.1.2.3",
			xfp:            "https",
			want:           true,
		},
		{
			name:           "trusted proxy says http",
			secureCookie:   true,
			trustedProxies: []string{"10.0.0.0/8"},
			remoteAddr:     "10.1.2.3",
			xfp:            "http",
			want:           false,
		},
		{
			name:           "trusted proxy proto list takes first value",
			secureCookie:   false,
			trustedProxies: []string{"10.0.0.0/8"},
			remoteAddr:     "10.1.2.3",
			xfp:            "https, http",
			want:           true,
		},
		{
			name:         "trusted proxy plain IP match",
			secureCookie: false,
			trustedProxies: []string{"192.168.1.10"},
			remoteAddr:     "192.168.1.10",
			xfp:            "https",
			want:           true,
		},
		{
			name:         "untrusted client spoofing X-Forwarded-Proto is ignored",
			secureCookie: true,
			remoteAddr:   "9.9.9.9",
			xfp:          "https",
			want:         true, // falls back to configured default, not the header
		},
		{
			name:           "untrusted peer header ignored even with proxies configured",
			secureCookie:   false,
			trustedProxies: []string{"10.0.0.0/8"},
			remoteAddr:     "9.9.9.9",
			xfp:            "https",
			want:           false, // untrusted peer: header ignored, falls back to false
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewAuthHandler(nil, tc.secureCookie, tc.trustedProxies, nil)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.xfp != "" {
				c.Request.Header.Set("X-Forwarded-Proto", tc.xfp)
			}
			if tc.remoteAddr != "" {
				c.Request.RemoteAddr = tc.remoteAddr + ":1234"
			}
			if tc.useTLS {
				c.Request.TLS = &tls.ConnectionState{}
			}
			if got := h.isSecure(c); got != tc.want {
				t.Fatalf("isSecure = %v, want %v", got, tc.want)
			}
		})
	}
}