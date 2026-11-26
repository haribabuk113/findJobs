package telemetry

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handler returns an HTTP handler that serves metrics in Prometheus text format.
func (m *Metrics) Handler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		snap := m.snapshot()
		var b strings.Builder
		for k, v := range snap.requests {
			fmt.Fprintf(&b, "http_requests_total{method=%q,route=%q,status=%q} %d\n", k[0], k[1], k[2], v)
		}
		for k, v := range snap.iamCalls {
			fmt.Fprintf(&b, "iam_calls_total{endpoint=%q} %d\n", k, v)
		}
		for k, v := range snap.iamFailures {
			fmt.Fprintf(&b, "iam_failures_total{endpoint=%q} %d\n", k, v)
		}
		for k, v := range snap.pgQueries {
			fmt.Fprintf(&b, "pg_queries_total{type=%q} %d\n", k, v)
		}
		for k, v := range snap.pgErrors {
			fmt.Fprintf(&b, "pg_query_errors_total{type=%q} %d\n", k, v)
		}
		for k, v := range snap.redisOps {
			fmt.Fprintf(&b, "redis_operations_total{command=%q} %d\n", k, v)
		}
		for k, v := range snap.redisErrors {
			fmt.Fprintf(&b, "redis_operation_errors_total{command=%q} %d\n", k, v)
		}
		fmt.Fprintf(&b, "redis_available %d\n", boolToInt(snap.redisAvailable))

		rt := snap.runtime
		fmt.Fprintf(&b, "# HELP go_goroutines Goroutines.\n# TYPE go_goroutines gauge\ngo_goroutines %d\n", rt.Goroutines)
		fmt.Fprintf(&b, "# HELP go_memstats_heap_alloc_bytes Heap in use.\n# TYPE go_memstats_heap_alloc_bytes gauge\ngo_memstats_heap_alloc_bytes %d\n", rt.HeapAlloc)
		fmt.Fprintf(&b, "# HELP go_memstats_heap_sys_bytes Heap sys.\n# TYPE go_memstats_heap_sys_bytes gauge\ngo_memstats_heap_sys_bytes %d\n", rt.HeapSys)
		fmt.Fprintf(&b, "# HELP go_memstats_heap_idle_bytes Heap idle.\n# TYPE go_memstats_heap_idle_bytes gauge\ngo_memstats_heap_idle_bytes %d\n", rt.HeapIdle)
		fmt.Fprintf(&b, "# HELP go_memstats_heap_inuse_bytes Heap in use.\n# TYPE go_memstats_heap_inuse_bytes gauge\ngo_memstats_heap_inuse_bytes %d\n", rt.HeapInuse)
		fmt.Fprintf(&b, "# HELP go_memstats_heap_released_bytes Heap released.\n# TYPE go_memstats_heap_released_bytes gauge\ngo_memstats_heap_released_bytes %d\n", rt.HeapReleased)
		fmt.Fprintf(&b, "# HELP go_memstats_heap_objects Heap objects.\n# TYPE go_memstats_heap_objects gauge\ngo_memstats_heap_objects %d\n", rt.HeapObjects)
		fmt.Fprintf(&b, "# HELP go_memstats_stack_inuse_bytes Stack in use.\n# TYPE go_memstats_stack_inuse_bytes gauge\ngo_memstats_stack_inuse_bytes %d\n", rt.StackInuse)
		fmt.Fprintf(&b, "# HELP go_memstats_stack_sys_bytes Stack sys.\n# TYPE go_memstats_stack_sys_bytes gauge\ngo_memstats_stack_sys_bytes %d\n", rt.StackSys)
		fmt.Fprintf(&b, "# HELP go_memstats_mspan_inuse_bytes Mspan in use.\n# TYPE go_memstats_mspan_inuse_bytes gauge\ngo_memstats_mspan_inuse_bytes %d\n", rt.MSpanInuse)
		fmt.Fprintf(&b, "# HELP go_memstats_mspan_sys_bytes Mspan sys.\n# TYPE go_memstats_mspan_sys_bytes gauge\ngo_memstats_mspan_sys_bytes %d\n", rt.MSpanSys)
		fmt.Fprintf(&b, "# HELP go_memstats_mcache_inuse_bytes Mcache in use.\n# TYPE go_memstats_mcache_inuse_bytes gauge\ngo_memstats_mcache_inuse_bytes %d\n", rt.MCacheInuse)
		fmt.Fprintf(&b, "# HELP go_memstats_mcache_sys_bytes Mcache sys.\n# TYPE go_memstats_mcache_sys_bytes gauge\ngo_memstats_mcache_sys_bytes %d\n", rt.MCacheSys)
		fmt.Fprintf(&b, "# HELP go_memstats_buck_hash_sys_bytes Buckhash sys.\n# TYPE go_memstats_buck_hash_sys_bytes gauge\ngo_memstats_buck_hash_sys_bytes %d\n", rt.BuckHashSys)
		fmt.Fprintf(&b, "# HELP go_memstats_gc_sys_bytes GCSys.\n# TYPE go_memstats_gc_sys_bytes gauge\ngo_memstats_gc_sys_bytes %d\n", rt.GCSys)
		fmt.Fprintf(&b, "# HELP go_memstats_other_sys_bytes OtherSys.\n# TYPE go_memstats_other_sys_bytes gauge\ngo_memstats_other_sys_bytes %d\n", rt.OtherSys)
		fmt.Fprintf(&b, "# HELP go_memstats_next_gc_bytes Next GC.\n# TYPE go_memstats_next_gc_bytes gauge\ngo_memstats_next_gc_bytes %d\n", rt.NextGC)
		fmt.Fprintf(&b, "# HELP go_memstats_last_gc_time_seconds Last GC time.\n# TYPE go_memstats_last_gc_time_seconds gauge\ngo_memstats_last_gc_time_seconds %s\n", strconv.FormatFloat(float64(rt.LastGC)/1e9, 'f', -1, 64))
		fmt.Fprintf(&b, "# HELP go_memstats_gc_cpu_fraction GOGC CPU fraction.\n# TYPE go_memstats_gc_cpu_fraction gauge\ngo_memstats_gc_cpu_fraction %g\n", rt.GCCPUFraction)
		fmt.Fprintf(&b, "# HELP go_memstats_mallocs_total Mallocs total.\n# TYPE go_memstats_mallocs_total counter\ngo_memstats_mallocs_total %d\n", rt.Mallocs)
		fmt.Fprintf(&b, "# HELP go_memstats_frees_total Frees total.\n# TYPE go_memstats_frees_total counter\ngo_memstats_frees_total %d\n", rt.Frees)
		fmt.Fprintf(&b, "# HELP go_memstats_alloc_bytes_total Total bytes allocated.\n# TYPE go_memstats_alloc_bytes_total counter\ngo_memstats_alloc_bytes_total %d\n", rt.TotalAlloc)
		fmt.Fprintf(&b, "# HELP go_memstats_sys_bytes Sys.\n# TYPE go_memstats_sys_bytes gauge\ngo_memstats_sys_bytes %d\n", rt.Sys)
		fmt.Fprintf(&b, "# HELP go_memstats_lookups_total Lookups total.\n# TYPE go_memstats_lookups_total counter\ngo_memstats_lookups_total %d\n", rt.Lookups)
		fmt.Fprintf(&b, "# HELP go_gc_count GC cycles.\n# TYPE go_gc_count counter\ngo_gc_count %d\n", rt.NumGC)
		fmt.Fprintf(&b, "# HELP go_gc_forced_count Forced GC cycles.\n# TYPE go_gc_forced_count counter\ngo_gc_forced_count %d\n", rt.NumForcedGC)
		fmt.Fprintf(&b, "# HELP go_gc_pause_total_seconds Total GC pause.\n# TYPE go_gc_pause_total_seconds counter\ngo_gc_pause_total_seconds %s\n", strconv.FormatFloat(float64(rt.PauseTotalNs)/1e9, 'f', -1, 64))
		fmt.Fprintf(&b, "# HELP go_gc_pause_last_seconds Last GC pause.\n# TYPE go_gc_pause_last_seconds gauge\ngo_gc_pause_last_seconds %s\n", strconv.FormatFloat(float64(rt.PauseNs)/1e9, 'f', -1, 64))
		fmt.Fprintf(&b, "# HELP go_memory_limit_bytes Soft memory limit from GOMEMLIMIT.\n# TYPE go_memory_limit_bytes gauge\ngo_memory_limit_bytes %d\n", parseMemLimit())
		fmt.Fprintf(&b, "# HELP go_goroutine_max_stack_bytes Max stack usage per goroutine.\n# TYPE go_goroutine_max_stack_bytes gauge\ngo_goroutine_max_stack_bytes %d\n", rt.MaxStackBytes)

		ctx.String(http.StatusOK, b.String())
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
