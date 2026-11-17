package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newProviderRetryFlagCmd(t *testing.T, flags map[string]string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "update"}
	cmd.Flags().Int32("provider-retry-max", 0, "")
	cmd.Flags().Int32("provider-retry-interval", 0, "")
	for name, value := range flags {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatal(err)
		}
	}
	return cmd
}

func TestApplyAgentProviderRetryFlags(t *testing.T) {
	t.Run("unset flags stay out of the body", func(t *testing.T) {
		body := map[string]any{}
		if err := applyAgentProviderRetryFlags(newProviderRetryFlagCmd(t, nil), body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 0 {
			t.Fatalf("body = %v, want empty", body)
		}
	})

	t.Run("set flags are sent", func(t *testing.T) {
		body := map[string]any{}
		cmd := newProviderRetryFlagCmd(t, map[string]string{
			"provider-retry-max":      "10",
			"provider-retry-interval": "1800",
		})
		if err := applyAgentProviderRetryFlags(cmd, body); err != nil {
			t.Fatal(err)
		}
		if body["provider_retry_max"] != int32(10) || body["provider_retry_interval_seconds"] != int32(1800) {
			t.Fatalf("body = %v, want 10 retries every 1800s", body)
		}
	})

	t.Run("zero turns retry off", func(t *testing.T) {
		body := map[string]any{}
		if err := applyAgentProviderRetryFlags(newProviderRetryFlagCmd(t, map[string]string{"provider-retry-max": "0"}), body); err != nil {
			t.Fatal(err)
		}
		if body["provider_retry_max"] != int32(0) {
			t.Fatalf("body = %v, want provider_retry_max 0", body)
		}
	})

	for _, tc := range []struct {
		flag, value, want string
	}{
		{"provider-retry-max", "21", "--provider-retry-max must be between 0 and 20"},
		{"provider-retry-max", "-1", "--provider-retry-max must be between 0 and 20"},
		{"provider-retry-interval", "29", "--provider-retry-interval must be between 30 and 86400"},
		{"provider-retry-interval", "86401", "--provider-retry-interval must be between 30 and 86400"},
	} {
		t.Run("rejects "+tc.flag+"="+tc.value, func(t *testing.T) {
			body := map[string]any{}
			err := applyAgentProviderRetryFlags(newProviderRetryFlagCmd(t, map[string]string{tc.flag: tc.value}), body)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if len(body) != 0 {
				t.Fatalf("rejected flag reached the body: %v", body)
			}
		})
	}
}
