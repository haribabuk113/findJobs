package telemetry

type metricsSnapshot struct {
	requests       map[[3]string]uint64
	latency        map[string]*histogramSnapshot
	iamCalls       map[string]uint64
	iamFailures    map[string]uint64
	iamLatency     map[string]*histogramSnapshot
	pgQueries      map[string]uint64
	pgErrors       map[string]uint64
	pgLatency      map[string]*histogramSnapshot
	redisOps       map[string]uint64
	redisErrors    map[string]uint64
	redisLatency   map[string]*histogramSnapshot
	redisAvailable bool
	runtime        runtimeStats
}

func (m *Metrics) snapshot() metricsSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := metricsSnapshot{
		requests:       map[[3]string]uint64{},
		latency:        map[string]*histogramSnapshot{},
		iamCalls:       map[string]uint64{},
		iamFailures:    map[string]uint64{},
		iamLatency:     map[string]*histogramSnapshot{},
		pgQueries:      map[string]uint64{},
		pgErrors:       map[string]uint64{},
		pgLatency:      map[string]*histogramSnapshot{},
		redisOps:       map[string]uint64{},
		redisErrors:    map[string]uint64{},
		redisLatency:   map[string]*histogramSnapshot{},
		redisAvailable: m.redisAvailable,
	}
	m.collectRuntime()
	out.runtime = m.runtime
	for k, v := range m.requests {
		out.requests[k] = v
	}
	for r, h := range m.latency {
		cp := make([]uint64, len(h.counts))
		copy(cp, h.counts)
		out.latency[r] = &histogramSnapshot{counts: cp, sum: h.sum, total: h.total}
	}
	for k, v := range m.iamCalls {
		out.iamCalls[k] = v
	}
	for k, v := range m.iamFailures {
		out.iamFailures[k] = v
	}
	for r, h := range m.iamLatency {
		cp := make([]uint64, len(h.counts))
		copy(cp, h.counts)
		out.iamLatency[r] = &histogramSnapshot{counts: cp, sum: h.sum, total: h.total}
	}
	for k, v := range m.pgQueries {
		out.pgQueries[k] = v
	}
	for k, v := range m.pgErrors {
		out.pgErrors[k] = v
	}
	for r, h := range m.pgLatency {
		cp := make([]uint64, len(h.counts))
		copy(cp, h.counts)
		out.pgLatency[r] = &histogramSnapshot{counts: cp, sum: h.sum, total: h.total}
	}
	for k, v := range m.redisOps {
		out.redisOps[k] = v
	}
	for k, v := range m.redisErrors {
		out.redisErrors[k] = v
	}
	for r, h := range m.redisLatency {
		cp := make([]uint64, len(h.counts))
		copy(cp, h.counts)
		out.redisLatency[r] = &histogramSnapshot{counts: cp, sum: h.sum, total: h.total}
	}
	return out
}
