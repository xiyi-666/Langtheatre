package service

import (
	"context"
	"log"
	"time"
)

// taskQueue isolates expensive model and TTS calls from request handlers.
// A bounded queue prevents a traffic spike from creating unlimited goroutines.
type taskQueue struct {
	jobs    chan func(context.Context)
	timeout time.Duration
}

func newTaskQueue(concurrency int, timeout time.Duration) *taskQueue {
	if concurrency <= 0 {
		concurrency = 30
	}
	if timeout <= 0 {
		timeout = 20 * time.Minute
	}
	queue := &taskQueue{
		jobs:    make(chan func(context.Context), concurrency*4),
		timeout: timeout,
	}
	for range concurrency {
		go queue.worker()
	}
	return queue
}

func (q *taskQueue) enqueue(job func(context.Context)) bool {
	select {
	case q.jobs <- job:
		return true
	default:
		return false
	}
}

func (q *taskQueue) worker() {
	for job := range q.jobs {
		ctx, cancel := context.WithTimeout(context.Background(), q.timeout)
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Printf("background task panic: %v", recovered)
				}
			}()
			job(ctx)
		}()
		cancel()
	}
}
