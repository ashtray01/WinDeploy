package deploy

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"windeploy/core/engine"
	"windeploy/core/inventory"
)

type fakeAdapter struct {
	executed []string
	failID   string
}

func (a *fakeAdapter) Precheck(context.Context, engine.Task) (engine.TaskStatus, error) {
	return engine.Pending, nil
}

func (a *fakeAdapter) Execute(_ context.Context, task engine.Task) (bool, error) {
	a.executed = append(a.executed, task.ID)
	if task.ID == a.failID {
		return false, errors.New("boom")
	}
	return false, nil
}

func (a *fakeAdapter) Verify(context.Context, engine.Task) error { return nil }

func sampleTasks() []engine.Task {
	return []engine.Task{
		{ID: "one", Name: "One", Category: "packages", Status: engine.Pending},
		{ID: "two", Name: "Two", Category: "packages", Status: engine.Pending, Dependencies: []string{"one"}},
	}
}

func newRunner(t *testing.T, adapter engine.Adapter) (Runner, string) {
	t.Helper()
	dir := t.TempDir()
	reportDir := t.TempDir()
	return Runner{
		Adapter: adapter, Store: Store{Dir: dir}, ProfileID: "office", ProfileName: "Офис",
		System: inventory.SystemInfo{Hostname: "PC-01"}, ReportDir: reportDir,
		// Session IDs derive from the wall clock; a fixed ID keeps parallel test
		// packages from colliding on the same second.
		SessionID:        "d20260101-000000-test",
		ReadBootID:       func(context.Context) (string, error) { return "boot-one", nil },
		CheckUserContext: func(context.Context) error { return nil },
		ReadSystem: func(context.Context) (inventory.SystemInfo, error) {
			return inventory.SystemInfo{Hostname: "PC-01", Architecture: "amd64", Windows: inventory.Version{Major: 10, Build: 26100, ProductType: 1}}, nil
		},
	}, dir
}

func TestStoreRoundTripAndSummary(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	tasks := sampleTasks()
	snapshot, err := engine.NewSnapshot(tasks, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	session := Session{Version: SessionVersion, ID: "d20260101-000000-abcd", ProfileID: "office", ProfileName: "Офис", CreatedAt: now, UpdatedAt: now, Tasks: tasks, Snapshot: snapshot}
	if err := store.Save(context.Background(), session); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	loaded, err := store.Load(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.ProfileID != "office" || len(loaded.Tasks) != 2 {
		t.Fatalf("unexpected session: %+v", loaded)
	}
	list, err := store.List(context.Background())
	if err != nil || len(list) != 1 {
		t.Fatalf("list failed: %+v, %v", list, err)
	}
	if list[0].Remaining != 2 || list[0].Completed {
		t.Fatalf("unexpected summary: %+v", list[0])
	}
	if err := store.Save(context.Background(), Session{Version: 1, ID: "../bad", Tasks: tasks}); err == nil {
		t.Fatal("invalid session accepted")
	}
}

func TestStartCreatesReportsAndCheckpoints(t *testing.T) {
	adapter := &fakeAdapter{}
	runner, dir := newRunner(t, adapter)
	session, err := runner.Start(context.Background(), sampleTasks())
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if !session.Completed || len(adapter.executed) != 2 {
		t.Fatalf("unexpected run: %+v executed=%v", session, adapter.executed)
	}
	if session.ReportJSON == "" || session.ReportHTML == "" {
		t.Fatalf("reports were not created: %+v", session)
	}
	for _, path := range []string{session.ReportJSON, session.ReportHTML} {
		if info, statErr := os.Stat(path); statErr != nil || info.Size() == 0 {
			t.Fatalf("report missing: %v %v", path, statErr)
		}
	}
	if _, statErr := os.Stat(dir + string(os.PathSeparator) + session.ID + ".json"); statErr != nil {
		t.Fatalf("session file missing: %v", statErr)
	}
	list, err := runner.Store.List(context.Background())
	if err != nil || len(list) != 1 || !list[0].Completed || list[0].Succeeded != 2 {
		t.Fatalf("unexpected summary: %+v, %v", list, err)
	}
}

func TestStartRecordsFailureAndResumeRetries(t *testing.T) {
	adapter := &fakeAdapter{failID: "two"}
	runner, _ := newRunner(t, adapter)
	session, err := runner.Start(context.Background(), sampleTasks())
	if err != nil {
		t.Fatal(err)
	}
	if !session.Completed || len(session.Snapshot.Results) != 2 {
		t.Fatalf("unexpected session: %+v", session)
	}
	failed := false
	for _, result := range session.Snapshot.Results {
		if result.TaskID == "two" && result.Status == engine.Failed {
			failed = true
		}
	}
	if !failed {
		t.Fatalf("expected failed task: %+v", session.Snapshot.Results)
	}
	if _, err := runner.Resume(context.Background(), session.ID, false); err == nil {
		t.Fatal("resume without retry must refuse completed session")
	}
	adapter.failID = ""
	resumed, err := runner.Resume(context.Background(), session.ID, true)
	if err != nil {
		t.Fatalf("retry resume failed: %v", err)
	}
	statuses := map[string]engine.TaskStatus{}
	for _, result := range resumed.Snapshot.Results {
		statuses[result.TaskID] = result.Status
	}
	if statuses["two"] != engine.Success {
		t.Fatalf("failed task was not retried: %+v", statuses)
	}
}

func TestResumeContinuesCancelledSession(t *testing.T) {
	adapter := &fakeAdapter{}
	runner, _ := newRunner(t, adapter)
	tasks := sampleTasks()
	snapshot, err := engine.NewSnapshot(tasks, []engine.TaskResult{{TaskID: "one", Status: engine.Cancelled, FinishedAt: time.Now().UTC()}, {TaskID: "two", Status: engine.Cancelled, FinishedAt: time.Now().UTC()}})
	if err != nil {
		t.Fatal(err)
	}
	session := Session{Version: SessionVersion, ID: NewSessionID(), ProfileID: "office", Tasks: tasks, Snapshot: snapshot, Cancelled: true}
	if err := runner.Store.Save(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	resumed, err := runner.Resume(context.Background(), session.ID, false)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if !resumed.Completed || len(adapter.executed) != 2 {
		t.Fatalf("unexpected resumed run: %+v executed=%v", resumed, adapter.executed)
	}
}

func TestRunnerRejectsInvalidInput(t *testing.T) {
	if _, err := (&Runner{}).Start(context.Background(), nil); err == nil {
		t.Fatal("runner without adapter accepted")
	}
	runner, _ := newRunner(t, &fakeAdapter{})
	if _, err := runner.Start(context.Background(), []engine.Task{{ID: "bad", Status: engine.Success}}); err == nil {
		t.Fatal("non-Pending task accepted")
	}
}
