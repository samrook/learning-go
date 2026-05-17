package pool_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"learning-go/bounded-worker-pool/pool"
)

func TestPool_BoundedConcurrency(test *testing.T) {
	test.Parallel()

	workerPool, err := pool.New(pool.Config{Workers: 4, QueueSize: 32})
	if err != nil {
		test.Fatal(err)
	}
	workerPool.Start()
	test.Cleanup(func() { _ = workerPool.Shutdown(context.Background()) })

	var current atomic.Int64
	var max atomic.Int64

	startedCh := make(chan struct{}, 10)
	releaseCh := make(chan struct{})

	job := func(ctx context.Context) error {
		n := current.Add(1)
		for {
			m := max.Load()
			if n <= m {
				break
			}
			if max.CompareAndSwap(m, n) {
				break
			}
		}
		startedCh <- struct{}{}
		select {
		case <-ctx.Done():
			current.Add(-1)
			return ctx.Err()
		case <-releaseCh:
			current.Add(-1)
			return nil
		}
	}

	for i := 0; i < 10; i++ {
		if err := workerPool.Submit(context.Background(), job); err != nil {
			test.Fatalf("submit %d: %v", i, err)
		}
	}

	waitForN(test, startedCh, 4, 2*time.Second)
	if got := current.Load(); got != 4 {
		test.Fatalf("current running = %d, want 4", got)
	}
	if got := max.Load(); got != 4 {
		test.Fatalf("max running = %d, want 4", got)
	}

	select {
	case <-startedCh:
		test.Fatalf("unexpected extra job started; pool not bounded")
	case <-time.After(75 * time.Millisecond):
	}

	close(releaseCh)
	waitForN(test, startedCh, 6, 2*time.Second) // remaining jobs should start eventually
}

func TestPool_Submit_BackpressureHonorsContext(test *testing.T) {
	test.Parallel()

	workerPool, err := pool.New(pool.Config{Workers: 1, QueueSize: 1})
	if err != nil {
		test.Fatal(err)
	}
	workerPool.Start()
	test.Cleanup(func() { _ = workerPool.Shutdown(context.Background()) })

	job1StartedCh := make(chan struct{}, 1)
	job1ReleaseCh := make(chan struct{})
	job1 := func(ctx context.Context) error {
		job1StartedCh <- struct{}{}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-job1ReleaseCh:
			return nil
		}
	}

	if err := workerPool.Submit(context.Background(), job1); err != nil {
		test.Fatal(err)
	}
	waitForN(test, job1StartedCh, 1, 2*time.Second)

	if err := workerPool.Submit(context.Background(), func(context.Context) error { return nil }); err != nil {
		test.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	err = workerPool.Submit(ctx, func(context.Context) error { return nil })
	if !errors.Is(err, context.DeadlineExceeded) {
		test.Fatalf("Submit err = %v, want context deadline exceeded", err)
	}

	close(job1ReleaseCh)
}

func TestPool_TrySubmit_RejectedWhenQueueFull(test *testing.T) {
	test.Parallel()

	workerPool, err := pool.New(pool.Config{Workers: 1, QueueSize: 1})
	if err != nil {
		test.Fatal(err)
	}
	workerPool.Start()
	test.Cleanup(func() { _ = workerPool.Shutdown(context.Background()) })

	startedCh := make(chan struct{}, 1)
	releaseCh := make(chan struct{})
	blocking := func(ctx context.Context) error {
		startedCh <- struct{}{}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-releaseCh:
			return nil
		}
	}

	if err := workerPool.Submit(context.Background(), blocking); err != nil {
		test.Fatal(err)
	}
	waitForN(test, startedCh, 1, 2*time.Second)

	if err := workerPool.Submit(context.Background(), func(context.Context) error { return nil }); err != nil {
		test.Fatal(err)
	}

	if ok := workerPool.TrySubmit(func(context.Context) error { return nil }); ok {
		test.Fatalf("TrySubmit ok = true, want false (queue full)")
	}

	snapshot := workerPool.Snapshot()
	if snapshot.JobsSubmitRejected != 1 {
		test.Fatalf("JobsSubmitRejected = %d, want 1", snapshot.JobsSubmitRejected)
	}

	close(releaseCh)
}

func waitForN(test *testing.T, ch <-chan struct{}, n int, timeout time.Duration) {
	test.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for i := range n {
		select {
		case <-ch:
		case <-deadline.C:
			test.Fatalf("timed out waiting for %d signals; got %d", n, i)
		}
	}
}
