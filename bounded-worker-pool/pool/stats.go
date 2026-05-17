package pool

import "sync/atomic"

type stats struct {
	workers atomic.Int64

	// workersRunning is a gauge for started-but-not-exited worker goroutines.
	workersRunning atomic.Int64
	activeWorkers  atomic.Int64

	jobsSubmitted      atomic.Uint64
	jobsSubmitRejected atomic.Uint64
	jobsProcessed      atomic.Uint64
	jobsFailed         atomic.Uint64

	processingNanosTotal atomic.Int64
	processingCount      atomic.Uint64
	lastProcessingNanos  atomic.Int64
}

type Snapshot struct {
	Workers       int
	ActiveWorkers int64
	QueueDepth    int64

	JobsSubmitted      uint64
	JobsSubmitRejected uint64
	JobsProcessed      uint64
	JobsFailed         uint64

	ProcessingNanosTotal int64
	ProcessingCount      uint64
	LastProcessingNanos  int64
}

func (snapshot Snapshot) AvgProcessingNanos() int64 {
	if snapshot.ProcessingCount == 0 {
		return 0
	}
	return snapshot.ProcessingNanosTotal / int64(snapshot.ProcessingCount)
}
