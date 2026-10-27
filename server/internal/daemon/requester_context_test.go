package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteRequesterContext(t *testing.T) {
	root := t.TempDir()
	env := map[string]string{}
	task := Task{ID: "t1", InitiatorName: "张家昌", InitiatorEmail: "zhangjc9@haidilao.com"}
	if err := writeRequesterContext(root, task, env); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, RequesterContextFile))
	if err != nil {
		t.Fatal(err)
	}
	var got requesterContext
	if err := json.Unmarshal(data, &got); err != nil || got.Name != "张家昌" || got.Email != "zhangjc9@haidilao.com" || got.TaskID != "t1" {
		t.Fatalf("got %+v err %v", got, err)
	}
	if env["MULTICA_REQUESTER_EMAIL"] != "zhangjc9@haidilao.com" || env["MULTICA_REQUESTER_NAME"] != "张家昌" {
		t.Fatalf("env %v", env)
	}
	// a later run on the same root without a human removes the stale file
	if err := writeRequesterContext(root, Task{ID: "t2"}, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, RequesterContextFile)); !os.IsNotExist(err) {
		t.Fatalf("stale requester file kept: %v", err)
	}
	// header-injection shaped values are refused
	if n, _ := requesterIdentity(Task{InitiatorName: "a>b", InitiatorEmail: "x@y"}); n != "" {
		t.Fatal("accepted unsafe name")
	}
}
