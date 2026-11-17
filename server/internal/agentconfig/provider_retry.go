package agentconfig

import "fmt"

// Per-agent retry for model-provider failures (GitHub #10815). A zero
// ProviderRetryMax disables the feature.
const (
	DefaultProviderRetryMax int32 = 0
	MinProviderRetryMax     int32 = 0
	MaxProviderRetryMax     int32 = 50

	DefaultProviderRetryIntervalSeconds int32 = 1800
	MinProviderRetryIntervalSeconds     int32 = 30
	MaxProviderRetryIntervalSeconds     int32 = 21600
)

func ValidateProviderRetryMax(value int32) error {
	if value < MinProviderRetryMax || value > MaxProviderRetryMax {
		return fmt.Errorf(
			"must be between %d and %d",
			MinProviderRetryMax,
			MaxProviderRetryMax,
		)
	}
	return nil
}

func ValidateProviderRetryIntervalSeconds(value int32) error {
	if value < MinProviderRetryIntervalSeconds || value > MaxProviderRetryIntervalSeconds {
		return fmt.Errorf(
			"must be between %d and %d",
			MinProviderRetryIntervalSeconds,
			MaxProviderRetryIntervalSeconds,
		)
	}
	return nil
}
