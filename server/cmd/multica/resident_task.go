package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// residentTaskContextFile mirrors daemon.ResidentTaskContextFile.
const residentTaskContextFile = ".multica_resident_task.json"

type residentTaskCredential struct {
	TaskID      string `json:"task_id"`
	AgentID     string `json:"agent_id"`
	WorkspaceID string `json:"workspace_id"`
	Token       string `json:"token"`
}

// residentTaskCredentialFromCwd returns the current task's own credential when
// running under a resident owner (MULTICA_RESIDENT_OWNER=1) inside a task tree,
// found by walking up from cwd to the daemon-written context file. Returns nil
// outside resident mode or when no valid file is found.
func residentTaskCredentialFromCwd() *residentTaskCredential {
	if os.Getenv("MULTICA_RESIDENT_OWNER") != "1" {
		return nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, residentTaskContextFile))
		if err == nil {
			var c residentTaskCredential
			if json.Unmarshal(data, &c) == nil && c.TaskID != "" && strings.HasPrefix(c.Token, "mat_") {
				return &c
			}
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}
