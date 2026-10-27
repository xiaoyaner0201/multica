package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ResidentTaskContextFile is written into a task's env root when the daemon
// serves a resident Hermes gateway (MULTICA_HERMES_RESIDENT_GATEWAY=1).
//
// A resident gateway is one long-lived process that runs tools for many tasks,
// so it never receives the per-task MULTICA_TASK_ID / mat_ token the daemon
// injects into a spawned agent CLI. Without them `multica attachment upload`
// cannot bind a generated file to the chat reply. The CLI, running under
// MULTICA_RESIDENT_OWNER=1 inside the task workdir, walks up from its cwd to
// this file and uses the task's own credential for that one call.
//
// The file is 0600, lives only for the task's lifetime (removed on exit), and
// the server revokes the mat_ token when the task reaches a terminal state.
const ResidentTaskContextFile = ".multica_resident_task.json"

type residentTaskContext struct {
	TaskID        string `json:"task_id"`
	AgentID       string `json:"agent_id"`
	WorkspaceID   string `json:"workspace_id"`
	ChatSessionID string `json:"chat_session_id,omitempty"`
	Token         string `json:"token"`
}

func residentGatewayEnabled() bool {
	return os.Getenv("MULTICA_HERMES_RESIDENT_GATEWAY") == "1"
}

// writeResidentTaskContext writes the context file and returns a cleanup func.
// It is a no-op unless the resident gateway mode is on and the provider is hermes.
func writeResidentTaskContext(envRoot, provider string, task Task, token string) (func(), error) {
	noop := func() {}
	if !residentGatewayEnabled() || provider != "hermes" || strings.TrimSpace(envRoot) == "" {
		return noop, nil
	}
	data, err := json.Marshal(residentTaskContext{
		TaskID:        task.ID,
		AgentID:       task.AgentID,
		WorkspaceID:   task.WorkspaceID,
		ChatSessionID: task.ChatSessionID,
		Token:         token,
	})
	if err != nil {
		return noop, err
	}
	path := filepath.Join(envRoot, ResidentTaskContextFile)
	tmp, err := os.CreateTemp(envRoot, ".multica_resident_task-*.tmp")
	if err != nil {
		return noop, err
	}
	tmpName := tmp.Name()
	if err := tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(data)
		if cerr := tmp.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Rename(tmpName, path)
		}
	} else {
		tmp.Close()
	}
	if err != nil {
		os.Remove(tmpName)
		return noop, err
	}
	return func() { os.Remove(path) }, nil
}
