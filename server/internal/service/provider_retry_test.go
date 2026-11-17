package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

const (
	providerRateLimit   = string(taskfailure.ReasonAgentProviderCapacityOrRateLimit)
	providerServerError = string(taskfailure.ReasonAgentProviderServerError)
)

type providerRetryFixture struct {
	pool      *pgxpool.Pool
	svc       *TaskService
	agentID   string
	runtimeID string
	issueID   string
	userID    string
}

func seedProviderRetryFixture(t *testing.T, maxRetries, intervalSeconds int32) *providerRetryFixture {
	t.Helper()
	pool := newResolveOriginatorPool(t)
	ctx := context.Background()
	_, userID, agentID, issueID := seedAttributionFixture(t, pool)
	var runtimeID string
	if err := pool.QueryRow(ctx, `
		UPDATE agent
		SET provider_retry_max = $2, provider_retry_interval_seconds = $3
		WHERE id = $1
		RETURNING runtime_id::text`, agentID, maxRetries, intervalSeconds).Scan(&runtimeID); err != nil {
		t.Fatalf("configure agent provider retry: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE issue SET status = 'in_progress' WHERE id = $1`, issueID); err != nil {
		t.Fatalf("activate issue: %v", err)
	}
	return &providerRetryFixture{
		pool:      pool,
		svc:       NewTaskService(db.New(pool), pool, nil, events.New()),
		agentID:   agentID,
		runtimeID: runtimeID,
		issueID:   issueID,
		userID:    userID,
	}
}

// runningTask inserts a running first attempt on the fixture issue.
func (f *providerRetryFixture) runningTask(t *testing.T) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	if err := f.pool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, issue_id, status, priority, attempt, max_attempts,
			originator_user_id, accountable_user_id, originator_source
		)
		VALUES ($1, $2, $3, 'running', 0, 1, 2, $4, $4, 'direct_human')
		RETURNING id`, f.agentID, f.runtimeID, f.issueID, f.userID).Scan(&id); err != nil {
		t.Fatalf("insert running task: %v", err)
	}
	t.Cleanup(func() {
		f.pool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE issue_id = $1`, f.issueID)
	})
	return id
}

func (f *providerRetryFixture) fail(t *testing.T, taskID pgtype.UUID, reason string) {
	t.Helper()
	if _, err := f.svc.FailTask(context.Background(), taskID, "provider said no", "sess", "/tmp/wd", "", reason, false, "", ""); err != nil {
		t.Fatalf("FailTask(%s): %v", reason, err)
	}
}

// retryChild returns the automatic retry of taskID, or nil when none exists.
func (f *providerRetryFixture) retryChild(t *testing.T, taskID pgtype.UUID) *db.AgentTaskQueue {
	t.Helper()
	var childID pgtype.UUID
	err := f.pool.QueryRow(context.Background(),
		`SELECT id FROM agent_task_queue WHERE retry_of_task_id = $1`, taskID).Scan(&childID)
	if err != nil {
		return nil
	}
	child, err := f.svc.Queries.GetAgentTask(context.Background(), childID)
	if err != nil {
		t.Fatalf("load retry child: %v", err)
	}
	return &child
}

// startChild moves a deferred retry straight to running, standing in for
// promotion and claim.
func (f *providerRetryFixture) startChild(t *testing.T, childID pgtype.UUID) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE agent_task_queue SET status = 'running', started_at = now() WHERE id = $1`, childID); err != nil {
		t.Fatalf("start retry child: %v", err)
	}
}

func (f *providerRetryFixture) failureComments(t *testing.T, taskID pgtype.UUID) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), `
		SELECT count(*) FROM comment
		WHERE issue_id = $1 AND type = 'system' AND source_task_id = $2`, f.issueID, taskID).Scan(&n); err != nil {
		t.Fatalf("count failure comments: %v", err)
	}
	return n
}

func TestProviderRetryOffKeepsFailingFast(t *testing.T) {
	for _, reason := range []string{providerRateLimit, providerServerError} {
		t.Run(reason, func(t *testing.T) {
			f := seedProviderRetryFixture(t, 0, 1800)
			taskID := f.runningTask(t)
			f.fail(t, taskID, reason)
			if child := f.retryChild(t, taskID); child != nil {
				t.Fatalf("provider retry off: got retry child %s, want none", util.UUIDToString(child.ID))
			}
			if n := f.failureComments(t, taskID); n != 1 {
				t.Fatalf("failure comments = %d, want 1 (final failure is visible)", n)
			}
		})
	}
}

func TestProviderRetryDefersByIntervalUntilBudgetIsSpent(t *testing.T) {
	const interval = 600
	f := seedProviderRetryFixture(t, 2, interval)
	taskID := f.runningTask(t)

	// Alternate the two provider reasons: both draw from the same budget.
	reasons := []string{providerRateLimit, providerServerError}
	for i := 0; i < 2; i++ {
		before := time.Now()
		f.fail(t, taskID, reasons[i])
		child := f.retryChild(t, taskID)
		if child == nil {
			t.Fatalf("failure %d: want deferred retry, got none", i+1)
		}
		if child.Status != "deferred" || !child.FireAt.Valid {
			t.Fatalf("failure %d: child status=%q fire_at valid=%v, want deferred with fire_at", i+1, child.Status, child.FireAt.Valid)
		}
		wait := child.FireAt.Time.Sub(before)
		if wait < interval*time.Second-5*time.Second || wait > interval*time.Second+5*time.Second {
			t.Fatalf("failure %d: retry fires in %s, want about %ds", i+1, wait, interval)
		}
		if child.Attempt > child.MaxAttempts {
			t.Fatalf("failure %d: child attempt %d exceeds max_attempts %d", i+1, child.Attempt, child.MaxAttempts)
		}
		if n := f.failureComments(t, taskID); n != 0 {
			t.Fatalf("failure %d: failure comments = %d while a retry is pending, want 0", i+1, n)
		}
		f.startChild(t, child.ID)
		taskID = child.ID
	}

	f.fail(t, taskID, providerRateLimit)
	if child := f.retryChild(t, taskID); child != nil {
		t.Fatalf("budget of 2 spent: got a third retry %s", util.UUIDToString(child.ID))
	}
	if n := f.failureComments(t, taskID); n != 1 {
		t.Fatalf("final failure comments = %d, want 1", n)
	}
}

func TestProviderRetryCountRestartsAfterASuccess(t *testing.T) {
	f := seedProviderRetryFixture(t, 1, 30)
	first := f.runningTask(t)
	f.fail(t, first, providerServerError)
	child := f.retryChild(t, first)
	if child == nil {
		t.Fatal("first provider failure: want a retry")
	}
	f.startChild(t, child.ID)
	if _, err := f.svc.CompleteTask(context.Background(), child.ID, []byte(`{"output":"done"}`), "sess", "/tmp/wd", "", false, "", ""); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	// A later run on the same issue (a new comment, say) starts a new chain.
	next := f.runningTask(t)
	f.fail(t, next, providerServerError)
	if f.retryChild(t, next) == nil {
		t.Fatal("after a success the budget must restart: want a retry for the next failure")
	}
}

func TestProviderRetryLeavesOtherFailuresAlone(t *testing.T) {
	for _, reason := range []string{
		string(taskfailure.ReasonAgentProviderQuotaLimit),
		string(taskfailure.ReasonAgentProviderAuthOrAccess),
		string(taskfailure.ReasonAgentMissingConfig),
		string(taskfailure.ReasonAgentModelNotFoundOrUnavailable),
		string(taskfailure.ReasonAgentContextOverflow),
		string(taskfailure.ReasonAgentUnknown),
	} {
		t.Run(reason, func(t *testing.T) {
			f := seedProviderRetryFixture(t, 5, 30)
			taskID := f.runningTask(t)
			f.fail(t, taskID, reason)
			if child := f.retryChild(t, taskID); child != nil {
				t.Fatalf("%s must not be retried by the provider policy", reason)
			}
		})
	}

	// Infrastructure retries keep their own schedule: timeout retries at once.
	f := seedProviderRetryFixture(t, 5, 30)
	taskID := f.runningTask(t)
	f.fail(t, taskID, "timeout")
	child := f.retryChild(t, taskID)
	if child == nil || child.Status != "queued" || child.FireAt.Valid || child.MaxAttempts != 2 {
		t.Fatalf("timeout retry = %+v, want an immediate queued child with max_attempts 2", child)
	}
}

func TestProviderRetrySkipsAutopilotAndTriageRuns(t *testing.T) {
	f := seedProviderRetryFixture(t, 5, 30)
	taskID := f.runningTask(t)
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE agent_task_queue SET context = '{"type":"triage"}'::jsonb WHERE id = $1`, taskID); err != nil {
		t.Fatalf("mark triage: %v", err)
	}
	f.fail(t, taskID, providerServerError)
	if child := f.retryChild(t, taskID); child != nil {
		t.Fatal("a Triage run must not get a provider retry")
	}
}

func TestProviderRetrySweeperPathUsesTheSameBudget(t *testing.T) {
	f := seedProviderRetryFixture(t, 1, 30)
	taskID := f.runningTask(t)
	if _, err := f.pool.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = 'failed', completed_at = now(), failure_reason = $2, error = 'upstream 503'
		WHERE id = $1`, taskID, providerServerError); err != nil {
		t.Fatalf("fail task directly: %v", err)
	}
	parent, err := f.svc.Queries.GetAgentTask(context.Background(), taskID)
	if err != nil {
		t.Fatal(err)
	}
	child, err := f.svc.MaybeRetryFailedTask(context.Background(), parent)
	if err != nil || child == nil {
		t.Fatalf("MaybeRetryFailedTask = %v, %v; want a deferred retry", child, err)
	}
	if child.Status != "deferred" || !child.FireAt.Valid {
		t.Fatalf("sweeper retry status=%q fire_at valid=%v, want deferred", child.Status, child.FireAt.Valid)
	}

	f.startChild(t, child.ID)
	if _, err := f.pool.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = 'failed', completed_at = now(), failure_reason = $2, error = 'upstream 503'
		WHERE id = $1`, child.ID, providerServerError); err != nil {
		t.Fatalf("fail retry directly: %v", err)
	}
	failedChild, err := f.svc.Queries.GetAgentTask(context.Background(), child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := f.svc.MaybeRetryFailedTask(context.Background(), failedChild); err != nil || again != nil {
		t.Fatalf("MaybeRetryFailedTask after budget = %v, %v; want no retry", again, err)
	}
}

func TestProviderRetryNewRunSupersedesPendingRetry(t *testing.T) {
	f := seedProviderRetryFixture(t, 3, 30)
	taskID := f.runningTask(t)
	f.fail(t, taskID, providerRateLimit)
	child := f.retryChild(t, taskID)
	if child == nil || child.Status != "deferred" {
		t.Fatalf("want a deferred retry, got %+v", child)
	}

	// A new comment triggers a fresh run while the retry is waiting.
	if _, err := f.pool.Exec(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, originator_user_id, accountable_user_id, originator_source)
		VALUES ($1, $2, $3, 'queued', 0, $4, $4, 'direct_human')`, f.agentID, f.runtimeID, f.issueID, f.userID); err != nil {
		t.Fatalf("enqueue new run: %v", err)
	}
	f.svc.cancelSupersededDeferredRetries(context.Background(), []pgtype.UUID{util.MustParseUUID(f.runtimeID)})
	var status string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT status FROM agent_task_queue WHERE id = $1`, child.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "cancelled" {
		t.Fatalf("pending provider retry status = %q after a new run, want cancelled", status)
	}
}

func TestProviderRetryPendingDoesNotWakeDelegator(t *testing.T) {
	f, svc := seedDelegatedFailureFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx,
		`UPDATE agent SET provider_retry_max = 2, provider_retry_interval_seconds = 30 WHERE id = $1`, f.worker); err != nil {
		t.Fatalf("configure worker: %v", err)
	}
	failedID := f.insertWorkerTask(t, "running", "comment", 1, 2)
	if _, err := svc.FailTask(ctx, failedID, "upstream overloaded", "", "", "", providerRateLimit, false, "", ""); err != nil {
		t.Fatalf("FailTask: %v", err)
	}
	var retries int
	if err := f.pool.QueryRow(ctx,
		`SELECT count(*) FROM agent_task_queue WHERE retry_of_task_id = $1 AND status = 'deferred'`, failedID).Scan(&retries); err != nil {
		t.Fatal(err)
	}
	if retries != 1 {
		t.Fatalf("deferred retries = %d, want 1", retries)
	}
	if n := f.recoveryTasks(t, failedID); n != 0 {
		t.Fatalf("delegator recovery tasks = %d while a provider retry is pending, want 0", n)
	}
	var recoveryComments int
	if err := f.pool.QueryRow(ctx, `
		SELECT count(*) FROM comment
		WHERE type = 'progress_update' AND source_task_id = $1`, failedID).Scan(&recoveryComments); err != nil {
		t.Fatal(err)
	}
	if recoveryComments != 0 {
		t.Fatalf("delegator recovery comments = %d while a provider retry is pending, want 0", recoveryComments)
	}
}

func TestProviderRetryAndInfrastructureRetryKeepSeparateBudgets(t *testing.T) {
	t.Run("infrastructure retry first", func(t *testing.T) {
		f := seedProviderRetryFixture(t, 1, 30)
		taskID := f.runningTask(t)
		f.fail(t, taskID, "timeout")
		infra := f.retryChild(t, taskID)
		if infra == nil || infra.Attempt != 2 {
			t.Fatalf("timeout retry = %+v, want attempt 2", infra)
		}
		f.startChild(t, infra.ID)
		f.fail(t, infra.ID, providerServerError)
		if provider := f.retryChild(t, infra.ID); provider == nil || provider.Status != "deferred" {
			t.Fatalf("provider failure after an infrastructure retry = %+v, want a deferred retry", provider)
		}
	})

	t.Run("provider retry first", func(t *testing.T) {
		f := seedProviderRetryFixture(t, 1, 30)
		taskID := f.runningTask(t)
		f.fail(t, taskID, providerServerError)
		provider := f.retryChild(t, taskID)
		if provider == nil || provider.Status != "deferred" {
			t.Fatalf("provider retry = %+v, want deferred", provider)
		}
		// The run's one infrastructure retry is still available.
		f.startChild(t, provider.ID)
		f.fail(t, provider.ID, "timeout")
		infra := f.retryChild(t, provider.ID)
		if infra == nil || infra.Status != "queued" {
			t.Fatalf("timeout after a provider retry = %+v, want an immediate retry", infra)
		}
		// The provider budget of 1 is spent for this chain.
		f.startChild(t, infra.ID)
		f.fail(t, infra.ID, providerRateLimit)
		if child := f.retryChild(t, infra.ID); child != nil {
			t.Fatalf("provider budget spent: got retry %s", util.UUIDToString(child.ID))
		}
	})
}
