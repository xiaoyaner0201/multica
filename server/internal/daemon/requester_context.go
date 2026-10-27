package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// RequesterContextFile is written into every task's env root so tooling inside
// the task (the runtime image's git wrapper) can attribute commits to the human
// who authorized the run (originator), instead of the shared Git account.
// It is read by walking up from the repository directory, which also covers a
// resident gateway that never receives per-task environment variables.
const RequesterContextFile = ".multica_requester.json"

type requesterContext struct {
	TaskID string `json:"task_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

func requesterIdentity(task Task) (string, string) {
	name := strings.TrimSpace(task.InitiatorName)
	email := strings.TrimSpace(task.InitiatorEmail)
	if name == "" || email == "" || !strings.Contains(email, "@") ||
		strings.ContainsAny(name+email, "<>\"\\&\n\r\x00") {
		return "", ""
	}
	return name, email
}

// writeRequesterContext writes (or removes a stale) requester file and sets the
// matching env vars. A run without a resolvable human leaves no file, so the
// image's pre-push check refuses to publish its commits under a placeholder.
func writeRequesterContext(envRoot string, task Task, agentEnv map[string]string) error {
	if strings.TrimSpace(envRoot) == "" {
		return nil
	}
	path := filepath.Join(envRoot, RequesterContextFile)
	name, email := requesterIdentity(task)
	if name == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if agentEnv != nil {
		agentEnv["MULTICA_REQUESTER_NAME"] = name
		agentEnv["MULTICA_REQUESTER_EMAIL"] = email
	}
	data, err := json.Marshal(requesterContext{TaskID: task.ID, Name: name, Email: email})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(envRoot, ".multica_requester-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err = tmp.Chmod(0o644); err == nil {
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
	}
	return err
}
