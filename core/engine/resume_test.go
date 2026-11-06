package engine

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func completedResult(id string, status TaskStatus) TaskResult {
	now := time.Now().UTC()
	return TaskResult{TaskID: id, Status: status, StartedAt: now, FinishedAt: now, Attempts: 1}
}

func TestResumeSkipsCompletedAndRechecksInterrupted(t *testing.T) {
	tasks := []Task{planTask("a"), planTask("b", "a"), planTask("c", "b")}
	prior := completedResult("a", Success)
	running := TaskResult{TaskID: "b", Status: Running, StartedAt: time.Now().UTC(), Attempts: 1, RebootRequired: true}
	snapshot, err := NewSnapshot(tasks, []TaskResult{prior, running})
	if err != nil {
		t.Fatal(err)
	}
	a := &fakeAdapter{precheck: func(_ context.Context, task Task) (TaskStatus, error) {
		if task.ID == "b" {
			return AlreadyInstalled, nil
		}
		return Pending, nil
	}}
	results, err := Resume(context.Background(), tasks, a, snapshot, ResumeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(results[0], prior) || results[1].Status != AlreadyInstalled || results[1].Attempts != 2 || !results[1].RebootRequired || results[2].Status != Success {
		t.Fatalf("%+v", results)
	}
	if !reflect.DeepEqual(a.calls, []string{"b:precheck", "c:precheck", "c:execute", "c:verify"}) {
		t.Fatalf("duplicated work: %v", a.calls)
	}
}

func TestResumeFailedRequiresOptIn(t *testing.T) {
	tasks := []Task{planTask("a"), planTask("b", "a")}
	failed := completedResult("a", Failed)
	failed.Error = "execute failed"
	skipped := TaskResult{TaskID: "b", Status: Skipped, FinishedAt: time.Now().UTC(), Error: "dependency did not succeed"}
	snapshot, err := NewSnapshot(tasks, []TaskResult{failed, skipped})
	if err != nil {
		t.Fatal(err)
	}
	for _, retry := range []bool{false, true} {
		a := &fakeAdapter{}
		results, err := Resume(context.Background(), tasks, a, snapshot, ResumeOptions{RetryFailed: retry})
		if err != nil {
			t.Fatal(err)
		}
		if retry {
			if results[0].Status != Success || results[1].Status != Success || results[0].Attempts != 2 {
				t.Fatalf("%+v", results)
			}
		} else if results[0].Status != Failed || results[1].Status != Skipped || len(a.calls) != 0 {
			t.Fatalf("%+v %v", results, a.calls)
		}
	}
}

func TestResumeRejectsMismatchedAndInconsistentState(t *testing.T) {
	tasks := []Task{planTask("a"), planTask("b", "a")}
	snapshot, err := NewSnapshot(tasks, nil)
	if err != nil {
		t.Fatal(err)
	}
	changed := append([]Task{}, tasks...)
	changed[0].Parameters = map[string]string{"changed": "yes"}
	a := &fakeAdapter{}
	if _, err = Resume(context.Background(), changed, a, snapshot, ResumeOptions{}); err == nil {
		t.Fatal("changed plan accepted")
	}
	for _, results := range [][]TaskResult{{completedResult("unknown", Success)}, {completedResult("b", Success)}, {completedResult("a", RebootRequired)}} {
		bad := snapshot
		bad.Results = results
		if _, err = Resume(context.Background(), tasks, a, bad, ResumeOptions{}); err == nil {
			t.Fatalf("invalid snapshot accepted: %+v", bad)
		}
	}
	if len(a.calls) != 0 {
		t.Fatal("adapter invoked for invalid state")
	}
}

func TestResumeCancelledTasksAndCompletedPlan(t *testing.T) {
	tasks := []Task{planTask("a")}
	for _, status := range []TaskStatus{Cancelled, AlreadyInstalled, Success, NotApplicable, Skipped} {
		prior := completedResult("a", status)
		snapshot, err := NewSnapshot(tasks, []TaskResult{prior})
		if err != nil {
			t.Fatal(err)
		}
		a := &fakeAdapter{}
		results, err := Resume(context.Background(), tasks, a, snapshot, ResumeOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if status == Cancelled {
			if results[0].Status != Success || len(a.calls) != 3 {
				t.Fatalf("%+v %v", results, a.calls)
			}
		} else if !reflect.DeepEqual(results[0], prior) || len(a.calls) != 0 {
			t.Fatal("completed task rerun")
		}
	}
}
