package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"windeploy/core/engine"
	"windeploy/internal/trusttest"
)

// These tests cover the session trust boundary: bounded and strict parsing, identity
// binding, rejected path shapes and refusal to resume from untrusted storage.

func validSession(at time.Time) Session {
	tasks := sampleTasks()
	snapshot, err := engine.NewSnapshot(tasks, nil)
	if err != nil {
		panic(err)
	}
	return Session{
		Version: SessionVersion, ID: "d20260101-000000-trust", ProfileID: "office", ProfileName: "Офис",
		CreatedAt: at, UpdatedAt: at, Tasks: tasks, Snapshot: snapshot,
	}
}

func writeSessionFile(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsInvalidSessionIdentifiers(t *testing.T) {
	dir := trusttest.Dir(t)
	writeSessionFile(t, dir, "d20260101-000000-ok.json", []byte("{}"))
	store := Store{Dir: dir}
	for _, id := range []string{
		"", "../escape", `..\escape`, "d/../escape", "/absolute", `C:\temp\session`,
		"d20260101-000000-ok/../../escape", ".hidden", strings.Repeat("d", 65), "Upper-Case",
	} {
		if _, err := store.Load(context.Background(), id); err == nil {
			t.Fatalf("identifier %q accepted", id)
		}
		if _, err := store.LoadForMutation(context.Background(), id); err == nil {
			t.Fatalf("mutating load accepted identifier %q", id)
		}
	}
}

func TestLoadRejectsMalformedSessionDocuments(t *testing.T) {
	dir := trusttest.Dir(t)
	store := Store{Dir: dir}
	id := "d20260101-000000-trust"
	valid, err := json.Marshal(validSession(time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		data []byte
	}{
		{"truncated", valid[:len(valid)/2]},
		{"trailing value", append(append([]byte{}, valid...), []byte(`{"extra":true}`)...)},
		{"trailing garbage", append(append([]byte{}, valid...), []byte("garbage")...)},
		{"unknown field", []byte(strings.Replace(string(valid), `"version":`, `"unexpected":true,"version":`, 1))},
		{"empty", []byte{}},
		{"corrupt", []byte{0x00, 0x01, 0x02, 0x03}},
	} {
		t.Run(test.name, func(t *testing.T) {
			writeSessionFile(t, dir, id+".json", test.data)
			if _, err := store.Load(context.Background(), id); err == nil {
				t.Fatalf("%s session accepted", test.name)
			}
		})
	}
}

func TestLoadRejectsOversizedSessionBeforeDecoding(t *testing.T) {
	dir := trusttest.Dir(t)
	id := "d20260101-000000-trust"
	path := filepath.Join(dir, id+".json")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxSessionBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	file.Close()
	if _, err := (Store{Dir: dir}).Load(context.Background(), id); err == nil {
		t.Fatal("oversized session accepted")
	}
}

func TestLoadRejectsSessionIdentityMismatch(t *testing.T) {
	dir := trusttest.Dir(t)
	session := validSession(time.Now().UTC())
	session.ID = "d20260101-000000-other"
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	requested := "d20260101-000000-trust"
	writeSessionFile(t, dir, requested+".json", data)
	if _, err := (Store{Dir: dir}).Load(context.Background(), requested); err == nil {
		t.Fatal("session with a different identity accepted")
	}
}

func TestLoadRejectsUnsupportedProfileAndPlan(t *testing.T) {
	at := time.Now().UTC()
	for _, test := range []struct {
		name   string
		change func(*Session)
	}{
		{"uppercase profile", func(s *Session) { s.ProfileID = "Office" }},
		{"profile traversal", func(s *Session) { s.ProfileID = "../office" }},
		{"profile control characters", func(s *Session) { s.ProfileName = "Офис\r\nПодделка" }},
		{"too many tasks", func(s *Session) { s.Tasks = make([]engine.Task, maxSessionTasks+1) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			session := validSession(at)
			test.change(&session)
			if err := session.Validate(); err == nil {
				t.Fatalf("%s session accepted", test.name)
			}
		})
	}
}

func TestLoadAllowsReadOnlyAccessToUntrustedStorage(t *testing.T) {
	dir := trusttest.Dir(t)
	session := validSession(time.Now().UTC())
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	writeSessionFile(t, dir, session.ID+".json", data)
	untrusted := Store{Dir: dir, Trust: func(context.Context, string, bool) error {
		return errors.New("not trusted")
	}}
	if _, err := untrusted.Load(context.Background(), session.ID); err != nil {
		t.Fatalf("read-only load refused: %v", err)
	}
	if _, err := untrusted.LoadForMutation(context.Background(), session.ID); err == nil {
		t.Fatal("mutating load accepted untrusted storage")
	}
}

func TestResumeRefusesUntrustedStorageBeforeAnyChange(t *testing.T) {
	runner, _ := newRunner(t, &fakeAdapter{})
	session, err := runner.Start(context.Background(), sampleTasks())
	if err != nil {
		t.Fatal(err)
	}
	adapter := &fakeAdapter{}
	runner.Adapter = adapter
	runner.Store.Trust = func(context.Context, string, bool) error { return errors.New("not trusted") }
	if _, err := runner.Resume(context.Background(), session.ID, true); err == nil {
		t.Fatal("resume accepted untrusted storage")
	}
	if len(adapter.executed) != 0 {
		t.Fatalf("untrusted resume executed tasks: %v", adapter.executed)
	}
	if _, err := runner.Store.Load(context.Background(), session.ID); err != nil {
		t.Fatalf("session file damaged by refused resume: %v", err)
	}
}

func TestResumeRequiresValidSessionPath(t *testing.T) {
	runner, _ := newRunner(t, &fakeAdapter{})
	runner.Store.Trust = func(context.Context, string, bool) error { return nil }
	if _, err := runner.Resume(context.Background(), "../escape", false); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestResumeStillRequiresUserContextForUntrustedLoadedPlan(t *testing.T) {
	ctx := context.Background()
	runner, _ := newRunner(t, &fakeAdapter{})
	session, err := runner.Start(ctx, sampleTasks())
	if err != nil {
		t.Fatal(err)
	}
	adapter := &fakeAdapter{}
	runner.Adapter = adapter
	runner.Store.Trust = func(context.Context, string, bool) error { return nil }
	runner.CheckUserContext = func(context.Context) error { return errors.New("context mismatch") }
	if _, err := runner.Resume(ctx, session.ID, true); err == nil {
		t.Fatal("resume accepted a mismatched user context")
	}
	if len(adapter.executed) != 0 {
		t.Fatalf("mismatched context executed tasks: %v", adapter.executed)
	}
}
