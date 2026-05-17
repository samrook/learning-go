package telemetry

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"learning-go/bounded-worker-pool/pool"
)

// Format is Prometheus-inspired, but intentionally simple for demo purposes.
func Handler(workerPool *pool.Pool) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		snapshot := workerPool.Snapshot()

		responseWriter.Header().Set("Content-Type", "text/plain; charset=utf-8")

		// Gauges
		_, _ = fmt.Fprintf(responseWriter, "workerpool_workers %d\n", snapshot.Workers)
		_, _ = fmt.Fprintf(responseWriter, "workerpool_active_workers %d\n", snapshot.ActiveWorkers)
		_, _ = fmt.Fprintf(responseWriter, "workerpool_queue_depth %d\n", snapshot.QueueDepth)

		// Counters
		_, _ = fmt.Fprintf(responseWriter, "workerpool_jobs_submitted_total %d\n", snapshot.JobsSubmitted)
		_, _ = fmt.Fprintf(responseWriter, "workerpool_jobs_submit_rejected_total %d\n", snapshot.JobsSubmitRejected)
		_, _ = fmt.Fprintf(responseWriter, "workerpool_jobs_processed_total %d\n", snapshot.JobsProcessed)
		_, _ = fmt.Fprintf(responseWriter, "workerpool_jobs_failed_total %d\n", snapshot.JobsFailed)

		// Latency
		_, _ = fmt.Fprintf(responseWriter, "workerpool_processing_latency_seconds_sum %s\n", formatSecondsFromNanos(snapshot.ProcessingNanosTotal))
		_, _ = fmt.Fprintf(responseWriter, "workerpool_processing_latency_seconds_count %d\n", snapshot.ProcessingCount)
		_, _ = fmt.Fprintf(responseWriter, "workerpool_processing_latency_seconds_last %s\n", formatSecondsFromNanos(snapshot.LastProcessingNanos))
		_, _ = fmt.Fprintf(responseWriter, "workerpool_processing_latency_seconds_avg %s\n", formatSecondsFromNanos(snapshot.AvgProcessingNanos()))
	})
}

func formatSecondsFromNanos(ns int64) string {
	seconds := float64(time.Duration(ns)) / float64(time.Second)
	return strconv.FormatFloat(seconds, 'f', 6, 64)
}
