package pool

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type JobFunc func(context.Context) error

type Config struct {
	Workers   int
	QueueSize int
	Logger    *slog.Logger
}

type Pool struct {
	workers int
	jobs    chan workItem

	log *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc

	startOnce sync.Once
	stopOnce  sync.Once
	waitGroup sync.WaitGroup

	closedCh chan struct{}

	stats stats

	started atomic.Bool
}

type workItem struct {
	job JobFunc
}

func New(cfg Config) (*Pool, error) {
	if cfg.Workers <= 0 {
		return nil, fmt.Errorf("workers must be > 0")
	}
	if cfg.QueueSize < 0 {
		return nil, fmt.Errorf("queue size must be >= 0")
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())
	workerPool := &Pool{
		workers:  cfg.Workers,
		jobs:     make(chan workItem, cfg.QueueSize),
		log:      log,
		ctx:      ctx,
		cancel:   cancel,
		closedCh: make(chan struct{}),
	}
	workerPool.stats.workers.Store(int64(cfg.Workers))
	return workerPool, nil
}

func (p *Pool) Start() {
	p.startOnce.Do(func() {
		p.started.Store(true)
		for workerID := 0; workerID < p.workers; workerID++ {
			p.waitGroup.Add(1)
			go p.worker(workerID)
		}
	})
}

func (p *Pool) Shutdown(ctx context.Context) error {
	p.stopOnce.Do(func() {
		close(p.closedCh)
		p.cancel()
	})

	done := make(chan struct{})
	go func() {
		p.waitGroup.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (p *Pool) Submit(ctx context.Context, job JobFunc) error {
	if job == nil {
		return fmt.Errorf("job must not be nil")
	}
	if !p.started.Load() {
		return ErrNotStarted
	}

	select {
	case <-p.closedCh:
		return ErrClosed
	default:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.closedCh:
		return ErrClosed
	case p.jobs <- workItem{job: job}:
		p.stats.jobsSubmitted.Add(1)
		return nil
	}
}

func (p *Pool) TrySubmit(job JobFunc) bool {
	if job == nil || !p.started.Load() {
		return false
	}
	select {
	case <-p.closedCh:
		return false
	default:
	}

	select {
	case p.jobs <- workItem{job: job}:
		p.stats.jobsSubmitted.Add(1)
		return true
	default:
		// queue full
		p.stats.jobsSubmitRejected.Add(1)
		return false
	}
}

func (p *Pool) QueueDepth() int { return len(p.jobs) }

func (p *Pool) Snapshot() Snapshot {
	return Snapshot{
		Workers:              int(p.stats.workers.Load()),
		ActiveWorkers:        p.stats.activeWorkers.Load(),
		QueueDepth:           int64(len(p.jobs)),
		JobsSubmitted:        p.stats.jobsSubmitted.Load(),
		JobsSubmitRejected:   p.stats.jobsSubmitRejected.Load(),
		JobsProcessed:        p.stats.jobsProcessed.Load(),
		JobsFailed:           p.stats.jobsFailed.Load(),
		ProcessingNanosTotal: p.stats.processingNanosTotal.Load(),
		ProcessingCount:      p.stats.processingCount.Load(),
		LastProcessingNanos:  p.stats.lastProcessingNanos.Load(),
	}
}

func (p *Pool) worker(workerID int) {
	defer p.waitGroup.Done()
	p.stats.workersRunning.Add(1)
	defer p.stats.workersRunning.Add(-1)

	for {
		select {
		case <-p.ctx.Done():
			return
		case item := <-p.jobs:
			// If shutdown has begun, do not start new work (discard queued items).
			select {
			case <-p.ctx.Done():
				return
			default:
			}

			// When queue size is 0, reads can block; but if ctx is canceled we'll exit above.
			if item.job == nil {
				// should never happen
				continue
			}

			p.stats.activeWorkers.Add(1)
			startTime := time.Now()
			err := item.job(p.ctx)
			duration := time.Since(startTime)
			p.stats.activeWorkers.Add(-1)

			p.stats.processingCount.Add(1)
			p.stats.processingNanosTotal.Add(duration.Nanoseconds())
			p.stats.lastProcessingNanos.Store(duration.Nanoseconds())

			if err != nil {
				p.stats.jobsFailed.Add(1)
				p.log.Debug("job failed", "worker_id", workerID, "err", err)
				continue
			}
			p.stats.jobsProcessed.Add(1)
		}
	}
}
