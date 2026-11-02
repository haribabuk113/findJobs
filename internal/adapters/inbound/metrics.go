package inbound

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Metrics is a minimal Prometheus endpoint (text exposition format, written
// by hand: a few counters don't justify the client library).
// ponytail: one mutex around all series; move to prometheus/client_golang if
// we need more metric types or it shows up under load.
type Metrics struct {
	mu       sync.Mutex
	requests map[[3]string]uint64  // method, route, status -> count
	latency  map[string]*histogram // route -> request duration
	pool     func() *pgxpool.Stat
}

// latencyBuckets are the histogram's upper bounds, in seconds.
var latencyBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type histogram struct {
	counts []uint64 // per bucket, not cumulative
	sum    float64
	total  uint64
}

// NewMetrics creates the collector; pool reports the DB pool's state.
func NewMetrics(pool func() *pgxpool.Stat) *Metrics {
	return &Metrics{requests: map[[3]string]uint64{}, latency: map[string]*histogram{}, pool: pool}
}

// Middleware records each request under its route template (/jobs/:id, never
// the raw path, so ids can't explode the series). Unmatched paths share one.
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		ctx.Next()
		route := ctx.FullPath()
		if route == "" {
			route = "unmatched"
		}
		took := time.Since(start).Seconds()
		m.mu.Lock()
		defer m.mu.Unlock()
		m.requests[[3]string{ctx.Request.Method, route, strconv.Itoa(ctx.Writer.Status())}]++
		h := m.latency[route]
		if h == nil {
			h = &histogram{counts: make([]uint64, len(latencyBuckets))}
			m.latency[route] = h
		}
		for i, le := range latencyBuckets {
			if took <= le {
				h.counts[i]++
				break
			}
		}
		h.sum += took
		h.total++
	}
}

// Handler serves GET /metrics.
func (m *Metrics) Handler(ctx *gin.Context) {
	var b strings.Builder
	m.mu.Lock()
	b.WriteString("# HELP http_requests_total HTTP requests by method, route and status.\n# TYPE http_requests_total counter\n")
	keys := make([][3]string, 0, len(m.requests))
	for k := range m.requests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return strings.Join(keys[i][:], " ") < strings.Join(keys[j][:], " ") })
	for _, k := range keys {
		fmt.Fprintf(&b, "http_requests_total{method=%q,route=%q,status=%q} %d\n", k[0], k[1], k[2], m.requests[k])
	}
	b.WriteString("# HELP http_request_duration_seconds Request latency by route.\n# TYPE http_request_duration_seconds histogram\n")
	routes := make([]string, 0, len(m.latency))
	for r := range m.latency {
		routes = append(routes, r)
	}
	sort.Strings(routes)
	for _, r := range routes {
		h := m.latency[r]
		var cumulative uint64
		for i, le := range latencyBuckets {
			cumulative += h.counts[i]
			fmt.Fprintf(&b, "http_request_duration_seconds_bucket{route=%q,le=%q} %d\n", r, strconv.FormatFloat(le, 'g', -1, 64), cumulative)
		}
		fmt.Fprintf(&b, "http_request_duration_seconds_bucket{route=%q,le=\"+Inf\"} %d\n", r, h.total)
		fmt.Fprintf(&b, "http_request_duration_seconds_sum{route=%q} %g\nhttp_request_duration_seconds_count{route=%q} %d\n", r, h.sum, r, h.total)
	}
	m.mu.Unlock()

	if m.pool != nil {
		s := m.pool()
		for _, g := range []struct {
			name, help string
			value      int64
		}{
			{"db_pool_acquired_conns", "Connections in use.", int64(s.AcquiredConns())},
			{"db_pool_idle_conns", "Idle connections.", int64(s.IdleConns())},
			{"db_pool_total_conns", "Open connections.", int64(s.TotalConns())},
			{"db_pool_max_conns", "Pool size limit (DB_MAX_CONNS).", int64(s.MaxConns())},
		} {
			fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", g.name, g.help, g.name, g.name, g.value)
		}
		fmt.Fprintf(&b, "# HELP db_pool_empty_acquire_total Acquires that had to wait for a connection.\n# TYPE db_pool_empty_acquire_total counter\ndb_pool_empty_acquire_total %d\n", s.EmptyAcquireCount())
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	fmt.Fprintf(&b, "# HELP go_goroutines Goroutines.\n# TYPE go_goroutines gauge\ngo_goroutines %d\n", runtime.NumGoroutine())
	fmt.Fprintf(&b, "# HELP go_memstats_heap_alloc_bytes Heap in use.\n# TYPE go_memstats_heap_alloc_bytes gauge\ngo_memstats_heap_alloc_bytes %d\n", mem.HeapAlloc)

	ctx.Header("Cache-Control", "no-store")
	ctx.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(b.String()))
}
