package queue_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/jobs"
	"github.com/tsdlamongan/whcms/backend/internal/platform/queue"
	"github.com/tsdlamongan/whcms/backend/internal/ports"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testOpt(t *testing.T) asynq.RedisClientOpt {
	t.Helper()
	opt := queue.RedisOpt("localhost:6379", "", 14) // isolated queue test DB
	// Probe availability.
	insp := asynq.NewInspector(opt)
	defer insp.Close()
	if _, err := insp.Queues(); err != nil {
		t.Skipf("redis not available, skipping: %v", err)
	}
	return opt
}

func TestEnqueuerEnqueuesTask(t *testing.T) {
	opt := testOpt(t)
	client := queue.NewClient(opt)
	defer client.Close()
	insp := asynq.NewInspector(opt)
	defer insp.Close()
	_, _ = insp.DeleteAllPendingTasks("default")

	e := queue.NewEnqueuer(client)
	payload := jobs.ProvisionCreatePayload{ServiceID: 42}
	require.NoError(t, e.Enqueue(context.Background(), jobs.TypeProvisionCreate, payload))

	tasks, err := insp.ListPendingTasks("default")
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, jobs.TypeProvisionCreate, tasks[0].Type)
	assert.Equal(t, jobs.DefaultMaxRetry, tasks[0].MaxRetry)

	var got jobs.ProvisionCreatePayload
	require.NoError(t, json.Unmarshal(tasks[0].Payload, &got))
	assert.Equal(t, int64(42), got.ServiceID)

	_, _ = insp.DeleteAllPendingTasks("default")
}

func TestEnqueuerOptions(t *testing.T) {
	opt := testOpt(t)
	client := queue.NewClient(opt)
	defer client.Close()
	insp := asynq.NewInspector(opt)
	defer insp.Close()
	_, _ = insp.DeleteAllPendingTasks("critical")
	_, _ = insp.DeleteAllScheduledTasks("critical")

	e := queue.NewEnqueuer(client)
	err := e.Enqueue(context.Background(), jobs.TypeMailSend,
		jobs.MailSendPayload{EmailLogID: 7},
		ports.WithQueue("critical"),
		ports.WithMaxRetry(2),
		ports.WithProcessIn(time.Hour),
	)
	require.NoError(t, err)

	tasks, err := insp.ListScheduledTasks("critical")
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, jobs.TypeMailSend, tasks[0].Type)
	assert.Equal(t, 2, tasks[0].MaxRetry)

	_, _ = insp.DeleteAllScheduledTasks("critical")
}

func TestEnqueuerMarshalError(t *testing.T) {
	opt := testOpt(t)
	client := queue.NewClient(opt)
	defer client.Close()

	e := queue.NewEnqueuer(client)
	err := e.Enqueue(context.Background(), "bad:task", make(chan int))
	assert.Error(t, err)
}

func TestConstructorsDoNotPanic(t *testing.T) {
	opt := queue.RedisOpt("localhost:6379", "", 14)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := queue.NewServer(opt, 5, log)
	assert.NotNil(t, srv)
	sched := queue.NewScheduler(opt, log)
	assert.NotNil(t, sched)
}

func TestEnqueuerUniqueTTLOption(t *testing.T) {
	opt := testOpt(t)
	client := queue.NewClient(opt)
	defer client.Close()
	insp := asynq.NewInspector(opt)
	defer insp.Close()
	_, _ = insp.DeleteAllPendingTasks("default")

	// Unique TTL locks are keyed by payload and persist in redis independently
	// of DeleteAllPendingTasks, so use a fresh EmailLogID each run to avoid
	// colliding with a lock left behind by a previous run within the TTL.
	e := queue.NewEnqueuer(client)
	err := e.Enqueue(context.Background(), jobs.TypeMailSend,
		jobs.MailSendPayload{EmailLogID: time.Now().UnixNano()},
		ports.WithUniqueTTL(time.Minute),
	)
	require.NoError(t, err)

	tasks, err := insp.ListPendingTasks("default")
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, jobs.TypeMailSend, tasks[0].Type)

	_, _ = insp.DeleteAllPendingTasks("default")
}

// TestEnqueuerEnqueueTransportError forces asynq's EnqueueContext call to
// fail (unreachable redis) to exercise Enqueue's wrapped-error return path.
func TestEnqueuerEnqueueTransportError(t *testing.T) {
	badOpt := queue.RedisOpt("127.0.0.1:1", "", 0) // nothing listens here
	client := queue.NewClient(badOpt)
	defer client.Close()

	e := queue.NewEnqueuer(client)
	err := e.Enqueue(context.Background(), "queue-test:unreachable", map[string]int{"a": 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "queue: enqueue queue-test:unreachable")
}

// syncBuffer is a concurrency-safe io.Writer used to capture slog output
// produced by asynq's background goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitForLogContains(t *testing.T, buf *syncBuffer, substr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), substr) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for log to contain %q; got: %s", substr, buf.String())
}

// TestNewServerErrorHandlerLogsFailure drives a real asynq worker through
// NewServer's Config.ErrorHandler closure by enqueuing a task whose handler
// always fails, and asserting the failure gets logged.
func TestNewServerErrorHandlerLogsFailure(t *testing.T) {
	opt := testOpt(t)
	insp := asynq.NewInspector(opt)
	defer insp.Close()
	_, _ = insp.DeleteAllPendingTasks("default")

	buf := &syncBuffer{}
	log := slog.New(slog.NewTextHandler(buf, nil))
	srv := queue.NewServer(opt, 1, log)

	const taskType = "queue-test:always-fail"
	mux := asynq.NewServeMux()
	mux.HandleFunc(taskType, func(ctx context.Context, task *asynq.Task) error {
		return errors.New("boom")
	})
	require.NoError(t, srv.Start(mux))
	defer srv.Shutdown()

	client := queue.NewClient(opt)
	defer client.Close()
	_, err := client.Enqueue(asynq.NewTask(taskType, nil), asynq.MaxRetry(0), asynq.Queue("default"))
	require.NoError(t, err)

	waitForLogContains(t, buf, "job failed", 5*time.Second)
	assert.Contains(t, buf.String(), taskType)

	_, _ = insp.DeleteAllPendingTasks("default")
}

// TestNewSchedulerPostEnqueueLogsFailure drives a real asynq scheduler
// through NewScheduler's PostEnqueueFunc closure by pointing it at an
// unreachable redis so every periodic enqueue attempt fails, and asserts the
// failure gets logged.
func TestNewSchedulerPostEnqueueLogsFailure(t *testing.T) {
	badOpt := queue.RedisOpt("127.0.0.1:1", "", 0) // nothing listens here

	buf := &syncBuffer{}
	log := slog.New(slog.NewTextHandler(buf, nil))
	sched := queue.NewScheduler(badOpt, log)

	require.NoError(t, sched.Start())
	defer sched.Shutdown()

	_, err := sched.Register("@every 1s", asynq.NewTask("queue-test:sched", nil))
	require.NoError(t, err)

	waitForLogContains(t, buf, "scheduler enqueue failed", 10*time.Second)
}
