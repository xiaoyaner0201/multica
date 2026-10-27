package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteResidentTaskContext(t *testing.T) {
	task := Task{ID: "task-1", AgentID: "agent-1", WorkspaceID: "ws-1", ChatSessionID: "chat-1"}

	t.Run("off without resident gateway env", func(t *testing.T) {
		t.Setenv("MULTICA_HERMES_RESIDENT_GATEWAY", "")
		root := t.TempDir()
		cleanup, err := writeResidentTaskContext(root, "hermes", task, "mat_x")
		if err != nil {
			t.Fatal(err)
		}
		cleanup()
		if _, err := os.Stat(filepath.Join(root, ResidentTaskContextFile)); !os.IsNotExist(err) {
			t.Fatalf("file must not exist, stat err=%v", err)
		}
	})

	t.Run("off for non-hermes provider", func(t *testing.T) {
		t.Setenv("MULTICA_HERMES_RESIDENT_GATEWAY", "1")
		root := t.TempDir()
		if _, err := writeResidentTaskContext(root, "claude", task, "mat_x"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(root, ResidentTaskContextFile)); !os.IsNotExist(err) {
			t.Fatalf("file must not exist for claude, stat err=%v", err)
		}
	})

	t.Run("written 0600 and removed by cleanup", func(t *testing.T) {
		t.Setenv("MULTICA_HERMES_RESIDENT_GATEWAY", "1")
		root := t.TempDir()
		cleanup, err := writeResidentTaskContext(root, "hermes", task, "mat_x")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ResidentTaskContextFile)
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Fatalf("mode = %v, want 0600", fi.Mode().Perm())
		}
		var got residentTaskContext
		data, _ := os.ReadFile(path)
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.TaskID != "task-1" || got.AgentID != "agent-1" || got.WorkspaceID != "ws-1" || got.ChatSessionID != "chat-1" || got.Token != "mat_x" {
			t.Fatalf("unexpected content %+v", got)
		}
		cleanup()
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("cleanup must remove file, stat err=%v", err)
		}
	})
}
