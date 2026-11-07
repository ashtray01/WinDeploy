package deploy

import (
	"context"
	"errors"
	"testing"
	"windeploy/core/engine"
	"windeploy/core/usercontext"
)

func TestContextGateBeforeAnyStartResumePhase(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(map[bool]string{false: "start", true: "resume"}[resume], func(t *testing.T) {
			adapter := &phaseCounter{}
			runner, _ := newRunner(t, adapter)
			tasks := sampleTasks()
			tasks[0].Category = "diagnostics"
			runner.CheckUserContext = func(context.Context) error { return usercontext.ErrUnconfirmed }
			var err error
			if !resume {
				_, err = runner.Start(context.Background(), tasks)
			} else {
				snapshot, e := engine.NewSnapshot(tasks, nil)
				if e != nil {
					t.Fatal(e)
				}
				session := Session{Version: SessionVersion, ID: NewSessionID(), ProfileID: "test", Tasks: tasks, Snapshot: snapshot}
				if e := runner.Store.Save(context.Background(), session); e != nil {
					t.Fatal(e)
				}
				_, err = runner.Resume(context.Background(), session.ID, false)
			}
			if !errors.Is(err, usercontext.ErrUnconfirmed) || adapter.calls != 0 {
				t.Fatalf("context mismatch reached adapter: calls=%d err=%v", adapter.calls, err)
			}
		})
	}
}

func TestUserScopeClassification(t *testing.T) {
	for _, test := range []struct {
		category string
		params   map[string]string
		want     bool
	}{
		{"shares", nil, true}, {"packages", nil, true}, {"drivers", nil, false}, {"diagnostics", nil, false},
		{"windows", map[string]string{"setting": "explorer"}, true}, {"windows", map[string]string{"setting": "timezone"}, false},
		{"browsers", map[string]string{"scope": "machine"}, false}, {"browsers", map[string]string{"scope": "user"}, true},
		{"browsers", map[string]string{"scope": "machine", "webapps_json": "[]"}, true}, {"printers", map[string]string{"set_default": "true"}, true},
	} {
		if actual := RequiresUserContext(engine.Task{Category: test.category, Parameters: test.params}); actual != test.want {
			t.Fatalf("%s/%v actual=%v", test.category, test.params, actual)
		}
	}
}

func TestAdapterRechecksContextBeforeExecute(t *testing.T) {
	a, connector := shareAdapter()
	task := engine.Task{ID: "share", Category: "shares", Parameters: map[string]string{"unc": `\\server\share`}}
	if _, err := a.Precheck(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	a.CheckUserContext = func(context.Context) error { return usercontext.ErrUnconfirmed }
	if _, err := a.Execute(context.Background(), task); !errors.Is(err, usercontext.ErrUnconfirmed) || connector.requests != 0 {
		t.Fatalf("context changed before execute: requests=%d err=%v", connector.requests, err)
	}
}
