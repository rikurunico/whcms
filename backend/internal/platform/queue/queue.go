// Package queue implements ports.Enqueuer over asynq and provides the asynq
// client/server/scheduler constructors used by cmd/worker.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/jobs"
	"github.com/tsdlamongan/whcms/backend/internal/ports"

	"github.com/hibiken/asynq"
)

// RedisOpt builds the asynq Redis connection options.
func RedisOpt(addr, password string, db int) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{Addr: addr, Password: password, DB: db}
}

// NewClient builds the asynq client (used by the Enqueuer).
func NewClient(opt asynq.RedisClientOpt) *asynq.Client {
	return asynq.NewClient(opt)
}

// NewServer builds the asynq worker server.
func NewServer(opt asynq.RedisClientOpt, concurrency int, log *slog.Logger) *asynq.Server {
	return asynq.NewServer(opt, asynq.Config{
		Concurrency: concurrency,
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			log.ErrorContext(ctx, "job failed", "type", task.Type(), "error", err)
		}),
	})
}

// NewScheduler builds the asynq periodic scheduler (UTC).
func NewScheduler(opt asynq.RedisClientOpt, log *slog.Logger) *asynq.Scheduler {
	return asynq.NewScheduler(opt, &asynq.SchedulerOpts{
		Location: time.UTC,
		PostEnqueueFunc: func(info *asynq.TaskInfo, err error) {
			if err != nil {
				log.Error("scheduler enqueue failed", "error", err)
			}
		},
	})
}

// Enqueuer is the asynq-backed ports.Enqueuer.
type Enqueuer struct {
	client *asynq.Client
}

// NewEnqueuer builds an Enqueuer.
func NewEnqueuer(client *asynq.Client) *Enqueuer { return &Enqueuer{client: client} }

// Enqueue JSON-marshals payload and enqueues an asynq task. Default retry
// policy is jobs.DefaultMaxRetry with exponential backoff.
func (e *Enqueuer) Enqueue(ctx context.Context, taskType string, payload any, opts ...ports.JobOption) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("queue: marshal %s payload: %w", taskType, err)
	}

	resolved := ports.JobOptions{MaxRetry: jobs.DefaultMaxRetry}
	for _, o := range opts {
		o(&resolved)
	}

	asynqOpts := []asynq.Option{asynq.MaxRetry(resolved.MaxRetry)}
	if resolved.Queue != "" {
		asynqOpts = append(asynqOpts, asynq.Queue(resolved.Queue))
	}
	if resolved.ProcessIn > 0 {
		asynqOpts = append(asynqOpts, asynq.ProcessIn(resolved.ProcessIn))
	}
	if resolved.UniqueTTL > 0 {
		asynqOpts = append(asynqOpts, asynq.Unique(resolved.UniqueTTL))
	}

	if _, err := e.client.EnqueueContext(ctx, asynq.NewTask(taskType, raw), asynqOpts...); err != nil {
		return fmt.Errorf("queue: enqueue %s: %w", taskType, err)
	}
	return nil
}
