package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/agentconfig"
)

func validateAgentMaxConcurrentTasksFlag(value int32) error {
	if err := agentconfig.ValidateMaxConcurrentTasks(value); err != nil {
		return fmt.Errorf(
			"--max-concurrent-tasks %w (got %d)",
			err,
			value,
		)
	}
	return nil
}

// applyAgentProviderRetryFlags copies explicitly set provider retry flags into
// a create/update body, rejecting out-of-range values before any API request.
func applyAgentProviderRetryFlags(cmd *cobra.Command, body map[string]any) error {
	if cmd.Flags().Changed("provider-retry-max") {
		v, _ := cmd.Flags().GetInt32("provider-retry-max")
		if err := agentconfig.ValidateProviderRetryMax(v); err != nil {
			return fmt.Errorf("--provider-retry-max %w (got %d)", err, v)
		}
		body["provider_retry_max"] = v
	}
	if cmd.Flags().Changed("provider-retry-interval") {
		v, _ := cmd.Flags().GetInt32("provider-retry-interval")
		if err := agentconfig.ValidateProviderRetryIntervalSeconds(v); err != nil {
			return fmt.Errorf("--provider-retry-interval %w (got %d)", err, v)
		}
		body["provider_retry_interval_seconds"] = v
	}
	return nil
}
