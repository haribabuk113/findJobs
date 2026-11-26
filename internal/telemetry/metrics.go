package telemetry

import (
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

var latencyBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type histogram struct {
	counts []uint64 // per bucket, not cumulative
	sum    float64
	total  uint64
}

type histogramSnapshot struct {
	counts []uint64
	sum    float64
	total  uint64
}

// Metrics holds HTTP, IAM, Postgres and Redis metrics in Prometheus text format.
// ponytail: one mutex around all series; move to prometheus/client_golang if
// we need more metric types or it shows up under load.
type Metrics struct {
	mu       sync.Mutex
	runtimeMu sync.RWMutex
	requests map[[3]string]uint64  // method, route, status -> count
	latency  map[string]*histogram // route -> request duration

	// IAM
	iamCalls    map[string]uint64
	iamFailures map[string]uint64
	iamLatency  map[string]*histogram

	// Postgres
	pgQueries map[string]uint64
	pgErrors  map[string]uint64
	pgLatency map[string]*histogram

	// Redis
	redisOps       map[string]uint64
	redisErrors    map[string]uint64
	redisLatency   map[string]*histogram
	redisAvailable bool

	pool func() *pgxpool.Stat

	// Runtime stats collected at scrape time.
	runtime runtimeStats
}

// runtimeStats holds Go runtime statistics (memory, GC, goroutines, stacks).
type runtimeStats struct {
	HeapAlloc      uint64
	HeapSys        uint64
	HeapIdle       uint64
	HeapInuse      uint64
	HeapReleased   uint64
	HeapObjects    uint64
	StackInuse     uint64
	StackSys       uint64
	MSpanInuse     uint64
	MSpanSys       uint64
	MCacheInuse    uint64
	MCacheSys      uint64
	BuckHashSys    uint64
	GCSys          uint64
	OtherSys       uint64
	NextGC         uint64
	LastGC         uint64
	PauseTotalNs   uint64
	PauseNs        uint64
	NumGC          uint32
	NumForcedGC    uint32
	GCCPUFraction  float64
	Mallocs        uint64
	Frees          uint64
	TotalAlloc     uint64
	Sys            uint64
	Lookups        uint64
	Goroutines     int
	MaxStackBytes  uint64
}

var defaultMetrics = newMetrics(nil)

// newMetrics creates a fresh collector with an optional pool stat provider.
func newMetrics(pool func() *pgxpool.Stat) *Metrics {
	return &Metrics{
		requests:       map[[3]string]uint64{},
		latency:        map[string]*histogram{},
		iamCalls:       map[string]uint64{},
		iamFailures:    map[string]uint64{},
		iamLatency:     map[string]*histogram{},
		pgQueries:      map[string]uint64{},
		pgErrors:       map[string]uint64{},
		pgLatency:      map[string]*histogram{},
		redisOps:       map[string]uint64{},
		redisErrors:    map[string]uint64{},
		redisLatency:   map[string]*histogram{},
		redisAvailable: true,
		pool:           pool,
	}
}
// collectRuntime samples the Go runtime and stores the result in m.runtime.
// It is called under m.mu from snapshot() so callers hold the outer lock.
func (m *Metrics) collectRuntime() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	var pauseNs uint64
	// PauseNs is a ring buffer of the last 256 pause durations; grab the most
	// recent non-zero entry, falling back to slot 0.
	for i := len(ms.PauseNs) - 1; i >= 0; i-- {
		if ms.PauseNs[i] != 0 {
			pauseNs = ms.PauseNs[i]
			break
		}
	}
	if pauseNs == 0 {
		pauseNs = ms.PauseNs[0]
	}

	m.runtime = runtimeStats{
		HeapAlloc:      ms.HeapAlloc,
		HeapSys:        ms.HeapSys,
		HeapIdle:       ms.HeapIdle,
		HeapInuse:      ms.HeapInuse,
		HeapReleased:   ms.HeapReleased,
		HeapObjects:    ms.HeapObjects,
		StackInuse:     ms.StackInuse,
		StackSys:       ms.StackSys,
		MSpanInuse:     ms.MSpanInuse,
		MSpanSys:       ms.MSpanSys,
		MCacheInuse:    ms.MCacheInuse,
		MCacheSys:      ms.MCacheSys,
		BuckHashSys:    ms.BuckHashSys,
		GCSys:          ms.GCSys,
		OtherSys:       ms.OtherSys,
		NextGC:         ms.NextGC,
		LastGC:         ms.LastGC,
		PauseTotalNs:   ms.PauseTotalNs,
		PauseNs:        pauseNs,
		NumGC:          ms.NumGC,
		NumForcedGC:    ms.NumForcedGC,
		GCCPUFraction:  ms.GCCPUFraction,
		Mallocs:        ms.Mallocs,
		Frees:          ms.Frees,
		TotalAlloc:     ms.TotalAlloc,
		Sys:            ms.Sys,
		Lookups:        ms.Lookups,
		Goroutines:     runtime.NumGoroutine(),
		MaxStackBytes:  maxStackBytes(),
	}
}

// maxStackBytes returns a best-effort estimate of the largest per-goroutine
// stack trace captured by runtime.Stack. It is not exact memory usage, but it
// gives an operator a signal when goroutine stacks grow unusually deep.
func maxStackBytes() uint64 {
	const maxBuf = 1 << 20 // 1 MiB is plenty for a process dump
	buf := make([]byte, maxBuf)
	n := runtime.Stack(buf, true)
	if n == 0 {
		return 0
	}
	var maxSeg uint64
	start := 0
	for i := 0; i < n; i++ {
		if buf[i] == '\n' && i+1 < n && buf[i+1] != '\t' && buf[i+1] != ' ' {
			seg := uint64(i - start)
			if seg > maxSeg {
				maxSeg = seg
			}
			start = i + 1
		}
	}
	if seg := uint64(n - start); seg > maxSeg {
		maxSeg = seg
	}
	return maxSeg
}
// parseMemLimit returns the soft memory limit from GOMEMLIMIT (in bytes).
func parseMemLimit() uint64 {
	v := os.Getenv("GOMEMLIMIT")
	if v == "" {
		return 0
	}
	var limit uint64
	for i := 0; i < len(v); i++ {
		if v[i] < '0' || v[i] > '9' {
			break
		}
		limit = limit*10 + uint64(v[i]-'0')
	}
	return limit
}


// NewMetrics returns the process-wide metrics collector and sets its pool
// stat provider. The provider is called on each /metrics scrape to report
// db pool gauges.
func NewMetrics(pool func() *pgxpool.Stat) *Metrics {
	defaultMetrics.pool = pool
	return defaultMetrics
}

// GetMetrics returns the process-wide metrics collector. It is initialized
// lazily with a nil pool provider, so outbound packages (postgres, redis,
// iam) can record metrics before the HTTP layer wires the pool provider.
func GetMetrics() *Metrics {
	return defaultMetrics
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

// RecordIAMCall records an IAM call with its outcome and latency.
func (m *Metrics) RecordIAMCall(endpoint string, success bool, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.iamCalls[endpoint]++
	if !success {
		m.iamFailures[endpoint]++
	}
	took := duration.Seconds()
	h := m.iamLatency[endpoint]
	if h == nil {
		h = &histogram{counts: make([]uint64, len(latencyBuckets))}
		m.iamLatency[endpoint] = h
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

// RecordPGQuery records a Postgres query execution with its outcome and latency.
func (m *Metrics) RecordPGQuery(queryType string, success bool, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pgQueries[queryType]++
	if !success {
		m.pgErrors[queryType]++
	}
	took := duration.Seconds()
	h := m.pgLatency[queryType]
	if h == nil {
		h = &histogram{counts: make([]uint64, len(latencyBuckets))}
		m.pgLatency[queryType] = h
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

// RecordRedisOp records a Redis operation with its outcome and latency.
func (m *Metrics) RecordRedisOp(op string, success bool, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.redisOps[op]++
	if !success {
		m.redisErrors[op]++
	}
	took := duration.Seconds()
	h := m.redisLatency[op]
	if h == nil {
		h = &histogram{counts: make([]uint64, len(latencyBuckets))}
		m.redisLatency[op] = h
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

// SetRedisAvailable sets the Redis availability gauge.
func (m *Metrics) SetRedisAvailable(avail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.redisAvailable = avail
}
