package telemetry_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"learning-go/bounded-worker-pool/pool"
	"learning-go/bounded-worker-pool/telemetry"
)

func TestHandler_EmitsExpectedMetrics(test *testing.T) {
	test.Parallel()

	workerPool, err := pool.New(pool.Config{Workers: 2, QueueSize: 8})
	if err != nil {
		test.Fatal(err)
	}
	workerPool.Start()
	test.Cleanup(func() { _ = workerPool.Shutdown(context.Background()) })

	for i := 0; i < 3; i++ {
		if err := workerPool.Submit(context.Background(), func(context.Context) error {
			time.Sleep(5 * time.Millisecond)
			return nil
		}); err != nil {
			test.Fatal(err)
		}
	}

	waitForProcessed(test, workerPool, 3, 2*time.Second)

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/metrics", nil)
	telemetry.Handler(workerPool).ServeHTTP(responseRecorder, request)

	body := responseRecorder.Body.String()

	assertContainsLine(test, body, "workerpool_workers 2")
	assertContainsLine(test, body, "workerpool_jobs_submitted_total 3")
	assertContainsLine(test, body, "workerpool_jobs_processed_total 3")
	assertContainsLine(test, body, "workerpool_processing_latency_seconds_count 3")

	// Presence checks for the latency series (values are time-dependent).
	assertContainsPrefix(test, body, "workerpool_processing_latency_seconds_sum ")
	assertContainsPrefix(test, body, "workerpool_processing_latency_seconds_last ")
	assertContainsPrefix(test, body, "workerpool_processing_latency_seconds_avg ")
}

func waitForProcessed(test *testing.T, workerPool *pool.Pool, want uint64, timeout time.Duration) {
	test.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()

	for {
		if got := workerPool.Snapshot().JobsProcessed; got >= want {
			return
		}
		select {
		case <-deadline.C:
			test.Fatalf("timed out waiting for JobsProcessed >= %d; got %d", want, workerPool.Snapshot().JobsProcessed)
		case <-tick.C:
		}
	}
}

func assertContainsLine(test *testing.T, body, line string) {
	test.Helper()
	if !strings.Contains(body, line+"\n") {
		test.Fatalf("missing line %q in metrics:\n%s", line, body)
	}
}

func assertContainsPrefix(test *testing.T, body, prefix string) {
	test.Helper()
	for _, ln := range strings.Split(body, "\n") {
		if strings.HasPrefix(ln, prefix) {
			return
		}
	}
	test.Fatalf("missing metrics line with prefix %q in:\n%s", prefix, body)
}
