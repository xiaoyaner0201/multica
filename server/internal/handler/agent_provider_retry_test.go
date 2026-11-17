package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateAgent_ProviderRetryDefaultsAndBounds(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	tests := []struct {
		name         string
		fields       map[string]any
		wantCode     int
		wantMax      int32
		wantInterval int32
		wantError    string
	}{
		{name: "omitted is off with a 30 minute interval", wantCode: http.StatusCreated, wantMax: 0, wantInterval: 1800},
		{name: "explicit settings", fields: map[string]any{"provider_retry_max": 10, "provider_retry_interval_seconds": 1800}, wantCode: http.StatusCreated, wantMax: 10, wantInterval: 1800},
		{name: "bounds accepted", fields: map[string]any{"provider_retry_max": 50, "provider_retry_interval_seconds": 30}, wantCode: http.StatusCreated, wantMax: 50, wantInterval: 30},
		{name: "upper interval bound accepted", fields: map[string]any{"provider_retry_interval_seconds": 21600}, wantCode: http.StatusCreated, wantMax: 0, wantInterval: 21600},
		{name: "max above 50 rejected", fields: map[string]any{"provider_retry_max": 51}, wantCode: http.StatusBadRequest, wantError: "provider_retry_max must be between 0 and 50"},
		{name: "negative max rejected", fields: map[string]any{"provider_retry_max": -1}, wantCode: http.StatusBadRequest, wantError: "provider_retry_max must be between 0 and 50"},
		{name: "interval below 30 rejected", fields: map[string]any{"provider_retry_interval_seconds": 29}, wantCode: http.StatusBadRequest, wantError: "provider_retry_interval_seconds must be between 30 and 21600"},
		{name: "interval above six hours rejected", fields: map[string]any{"provider_retry_interval_seconds": 21601}, wantCode: http.StatusBadRequest, wantError: "provider_retry_interval_seconds must be between 30 and 21600"},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := map[string]any{
				"name":       fmt.Sprintf("provider-retry-create-%d", i),
				"runtime_id": handlerTestRuntimeID(t),
			}
			for k, v := range tt.fields {
				body[k] = v
			}

			w := httptest.NewRecorder()
			testHandler.CreateAgent(w, newRequest(http.MethodPost, "/api/agents", body))
			if w.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.wantCode, w.Body.String())
			}
			if tt.wantCode == http.StatusBadRequest {
				if !strings.Contains(w.Body.String(), tt.wantError) {
					t.Fatalf("error = %s, want %q", w.Body.String(), tt.wantError)
				}
				return
			}

			var response AgentResponse
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			t.Cleanup(func() {
				testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, response.ID)
			})
			if response.ProviderRetryMax != tt.wantMax || response.ProviderRetryIntervalSeconds != tt.wantInterval {
				t.Fatalf("response provider retry = %d/%ds, want %d/%ds",
					response.ProviderRetryMax, response.ProviderRetryIntervalSeconds, tt.wantMax, tt.wantInterval)
			}
		})
	}
}

func TestUpdateAgent_ProviderRetryBoundsAndOmission(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	agentID := createHandlerTestAgent(t, "provider-retry-update", nil)
	readPersisted := func() (int32, int32) {
		t.Helper()
		var maxRetries, interval int32
		if err := testPool.QueryRow(context.Background(),
			`SELECT provider_retry_max, provider_retry_interval_seconds FROM agent WHERE id = $1`, agentID,
		).Scan(&maxRetries, &interval); err != nil {
			t.Fatalf("read provider retry: %v", err)
		}
		return maxRetries, interval
	}
	update := func(t *testing.T, body map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		testHandler.UpdateAgent(w, withURLParam(newRequest(http.MethodPut, "/api/agents/"+agentID, body), "id", agentID))
		return w
	}

	if w := update(t, map[string]any{"provider_retry_max": 10, "provider_retry_interval_seconds": 600}); w.Code != http.StatusOK {
		t.Fatalf("set provider retry: status = %d: %s", w.Code, w.Body.String())
	}
	if maxRetries, interval := readPersisted(); maxRetries != 10 || interval != 600 {
		t.Fatalf("persisted = %d/%ds, want 10/600s", maxRetries, interval)
	}

	for _, body := range []map[string]any{
		{"provider_retry_max": 51},
		{"provider_retry_interval_seconds": 10},
		{"provider_retry_max": 5, "provider_retry_interval_seconds": 100000},
	} {
		if w := update(t, body); w.Code != http.StatusBadRequest {
			t.Fatalf("update %v: status = %d, want 400: %s", body, w.Code, w.Body.String())
		}
		if maxRetries, interval := readPersisted(); maxRetries != 10 || interval != 600 {
			t.Fatalf("rejected update %v persisted %d/%ds, want 10/600s", body, maxRetries, interval)
		}
	}

	if w := update(t, map[string]any{"description": "retry settings unchanged"}); w.Code != http.StatusOK {
		t.Fatalf("unrelated update: status = %d: %s", w.Code, w.Body.String())
	}
	if maxRetries, interval := readPersisted(); maxRetries != 10 || interval != 600 {
		t.Fatalf("omitted fields changed provider retry to %d/%ds, want 10/600s", maxRetries, interval)
	}

	if w := update(t, map[string]any{"provider_retry_max": 0}); w.Code != http.StatusOK {
		t.Fatalf("turn off: status = %d: %s", w.Code, w.Body.String())
	}
	if maxRetries, interval := readPersisted(); maxRetries != 0 || interval != 600 {
		t.Fatalf("turning off persisted %d/%ds, want 0/600s (interval kept)", maxRetries, interval)
	}
}
