// Package worker provides a goroutine-based worker pool for parallel badge
// processing (issuance, verification, etc.).
//
// Architecture notes
// ------------------
// * A fixed set of goroutine workers pull jobs from a shared buffered channel.
// * Each job carries its own result channel so callers can await completion.
// * The pool is context-aware: cancelling the context drains remaining jobs and
//   shuts workers down gracefully.
// * Submit() is non-blocking as long as the job queue is not full; if the queue
//   is full it will block until space is available or the context is cancelled.
package worker

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
)

// JobType distinguishes the kind of work a job represents.
type JobType string

const (
	JobTypeIssue  JobType = "issue"
	JobTypeVerify JobType = "verify"
)

// Job is a unit of work submitted to the pool.
type Job struct {
	ID         string
	Type       JobType
	Payload    interface{}
	resultChan chan<- JobResult // written once by the worker, then closed
}

// JobResult is the outcome of processing a single job.
type JobResult struct {
	JobID string
	Data  interface{}
	Error error
}

// ProcessFunc is a callback that processes a single job. The pool calls the
// registered ProcessFunc for each job type. If no function is registered for
// a given job type, the pool falls back to the built-in default handler.
type ProcessFunc func(job Job) JobResult

// Pool manages a fixed number of goroutine workers.
type Pool struct {
	workerCount int
	jobs        chan Job
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	processors  map[JobType]ProcessFunc
}

// NewPool creates a pool with the given worker count and queue size.
// If workerCount <= 0 the pool defaults to runtime.NumCPU() * 2 which is a
// reasonable starting point for mixed CPU / IO workloads.
func NewPool(workerCount, queueSize int) *Pool {
	if workerCount <= 0 {
		workerCount = runtime.NumCPU() * 2
	}
	if queueSize <= 0 {
		queueSize = 1000
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &Pool{
		workerCount: workerCount,
		jobs:        make(chan Job, queueSize),
		ctx:         ctx,
		cancel:      cancel,
		processors:  make(map[JobType]ProcessFunc),
	}
	return p
}

// RegisterProcessor registers a ProcessFunc for a given job type.
// This allows services to wire real processing logic into the pool.
func (p *Pool) RegisterProcessor(jobType JobType, fn ProcessFunc) {
	p.processors[jobType] = fn
}

// Start launches all workers. It must be called exactly once.
func (p *Pool) Start() {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	log.Printf("[worker-pool] started %d workers (queue capacity %d)", p.workerCount, cap(p.jobs))
}

// Submit enqueues a job and returns a channel the caller can read for the result.
// The returned channel will receive exactly one value and then be closed.
// Submit respects the pool's context: if the context is cancelled before the job
// can be enqueued the returned channel receives an error immediately.
func (p *Pool) Submit(id string, jobType JobType, payload interface{}) <-chan JobResult {
	rc := make(chan JobResult, 1)

	job := Job{
		ID:         id,
		Type:       jobType,
		Payload:    payload,
		resultChan: rc,
	}

	// Use select so we never block forever when the pool is shutting down.
	select {
	case p.jobs <- job:
		// enqueued successfully
	case <-p.ctx.Done():
		rc <- JobResult{
			JobID: id,
			Error: fmt.Errorf("worker pool shut down, job %s not accepted", id),
		}
		close(rc)
	}

	return rc
}

// Shutdown signals all workers to finish, waits for in-flight jobs to complete,
// and then returns. After Shutdown returns no new jobs will be accepted.
func (p *Pool) Shutdown() {
	// Signal no more jobs will be sent.
	p.cancel()
	close(p.jobs)
	// Wait for every worker to drain its current job.
	p.wg.Wait()
	log.Println("[worker-pool] all workers stopped")
}

// worker is the loop each goroutine runs. It reads from the jobs channel until
// the channel is closed (during shutdown), ensuring every accepted job gets
// processed even during graceful termination.
func (p *Pool) worker(id int) {
	defer p.wg.Done()
	log.Printf("[worker-%d] ready", id)

	for job := range p.jobs {
		result := p.process(job)
		job.resultChan <- result
		close(job.resultChan)
	}

	log.Printf("[worker-%d] stopped", id)
}

// process executes the job. If a ProcessFunc has been registered for the job
// type via RegisterProcessor, it is used; otherwise a default fallback runs.
func (p *Pool) process(job Job) JobResult {
	if fn, ok := p.processors[job.Type]; ok {
		return fn(job)
	}

	// Default fallback for unregistered job types.
	switch job.Type {
	case JobTypeIssue:
		return JobResult{JobID: job.ID, Data: map[string]string{"status": "issued (no processor registered)"}}
	case JobTypeVerify:
		return JobResult{JobID: job.ID, Data: map[string]string{"status": "verified (no processor registered)"}}
	default:
		return JobResult{JobID: job.ID, Error: fmt.Errorf("unknown job type: %s", job.Type)}
	}
}
