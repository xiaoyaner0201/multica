package handler

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/multica-ai/multica/server/internal/agentconfig"
)

func validateAgentMaxConcurrentTasks(value int32) error {
	if err := agentconfig.ValidateMaxConcurrentTasks(value); err != nil {
		return fmt.Errorf("max_concurrent_tasks %w", err)
	}
	return nil
}

func defaultAndValidateAgentMaxConcurrentTasks(rawFields map[string]json.RawMessage, value *int32) error {
	raw, provided := rawFields["max_concurrent_tasks"]
	if !provided || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		*value = agentconfig.DefaultMaxConcurrentTasks
		return nil
	}
	return validateAgentMaxConcurrentTasks(*value)
}

// validateAgentProviderRetry checks the provider retry settings a create or
// update request supplies. A nil field was not sent.
func validateAgentProviderRetry(maxRetries, intervalSeconds *int32) error {
	if maxRetries != nil {
		if err := agentconfig.ValidateProviderRetryMax(*maxRetries); err != nil {
			return fmt.Errorf("provider_retry_max %w", err)
		}
	}
	if intervalSeconds != nil {
		if err := agentconfig.ValidateProviderRetryIntervalSeconds(*intervalSeconds); err != nil {
			return fmt.Errorf("provider_retry_interval_seconds %w", err)
		}
	}
	return nil
}
