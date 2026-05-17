package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"learning-go/bounded-worker-pool/pool"
	"learning-go/bounded-worker-pool/telemetry"
)

func main() {
	var (
		addr    = flag.String("addr", ":8080", "HTTP listen address")
		workers = flag.Int("workers", 4, "number of worker goroutines")
		queue   = flag.Int("queue", 64, "job queue size (buffered channel)")
	)
	flag.Parse()

	workerPool, err := pool.New(pool.Config{Workers: *workers, QueueSize: *queue})
	if err != nil {
		log.Fatal(err)
	}
	workerPool.Start()
	defer func() {
		_ = workerPool.Shutdown(context.Background())
	}()

	serverMux := http.NewServeMux()
	serverMux.Handle("/metrics", telemetry.Handler(workerPool))
	serverMux.HandleFunc("/enqueue", func(w http.ResponseWriter, r *http.Request) {
		jobCount := intParam(r, "n", 1)
		jobDurationMillis := intParam(r, "ms", 25)
		if jobCount <= 0 || jobDurationMillis < 0 {
			http.Error(w, "invalid n/ms", http.StatusBadRequest)
			return
		}

		requestContext := r.Context()
		for i := 0; i < jobCount; i++ {
			jobDuration := time.Duration(jobDurationMillis) * time.Millisecond
			err := workerPool.Submit(requestContext, func(jobContext context.Context) error {
				sleepTimer := time.NewTimer(jobDuration)
				defer sleepTimer.Stop()
				select {
				case <-jobContext.Done():
					return jobContext.Err()
				case <-sleepTimer.C:
					return nil
				}
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
		}

		_, _ = fmt.Fprintf(w, "enqueued %d jobs\n", jobCount)
	})

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           serverMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", *addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(ctx)
	_ = workerPool.Shutdown(ctx)
}

func intParam(request *http.Request, key string, defaultValue int) int {
	rawValue := request.URL.Query().Get(key)
	if rawValue == "" {
		return defaultValue
	}
	parsedValue, err := strconv.Atoi(rawValue)
	if err != nil {
		return defaultValue
	}
	return parsedValue
}
