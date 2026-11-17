package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

// providerRetryReasons are the model-provider failures an agent can opt into
// retrying (GitHub #10815). They stay out of retryableReasons: by default a
// provider outage fails fast so the user sees it. Quota, auth, configuration,
// unknown-model and context-overflow failures are never retried, because
// waiting does not fix them.
var providerRetryReasons = map[string]bool{
	string(taskfailure.ReasonAgentProviderCapacityOrRateLimit): true,
	string(taskfailure.ReasonAgentProviderServerError):         true,
}

// providerRetryPlan is the deferred retry an agent's provider-retry settings
// call for.
type providerRetryPlan struct {
	fireAt      pgtype.Timestamptz
	maxAttempts pgtype.Int4
}

// planProviderRetry decides whether a run that failed with a model-provider
// error gets one more deferred attempt under its agent's settings.
//
// The budget is counted per automatic-retry chain: the ancestors of the failed
// run that also failed with a provider error. A chain only continues through
// failures, and a new comment, assignment or manual rerun starts a new chain,
// so any success resets the count. Infrastructure retries in the same chain do
// not consume it.
//
// Only issue runs qualify. A deferred chat retry would hold back the user's
// next turn for the whole interval, and autopilot and Triage runs have their
// own recovery, exactly as for infrastructure retries. A task whose
// max_attempts disables auto-retry stays disabled.
func planProviderRetry(
	ctx context.Context,
	q *db.Queries,
	failureReason string,
	failed db.AgentTaskQueue,
	agent db.Agent,
	now time.Time,
) (providerRetryPlan, bool, error) {
	if !providerRetryReasons[failureReason] ||
		agent.ProviderRetryMax <= 0 ||
		failed.MaxAttempts <= 1 ||
		!failed.IssueID.Valid ||
		failed.AutopilotRunID.Valid ||
		IsTriageTask(failed) {
		return providerRetryPlan{}, false, nil
	}
	used, err := q.CountProviderRetryAncestors(ctx, failed.ID)
	if err != nil {
		return providerRetryPlan{}, false, fmt.Errorf("count provider retries: %w", err)
	}
	if used >= agent.ProviderRetryMax {
		return providerRetryPlan{}, false, nil
	}
	interval := time.Duration(agent.ProviderRetryIntervalSeconds) * time.Second
	return providerRetryPlan{
		fireAt: pgtype.Timestamptz{Time: now.Add(interval), Valid: true},
		// Raise the ceiling by one so this retry does not eat into the budget
		// infrastructure retries have later in the chain, and the child row
		// stays self-consistent (attempt <= max_attempts).
		maxAttempts: pgtype.Int4{Int32: failed.MaxAttempts + 1, Valid: true},
	}, true, nil
}
