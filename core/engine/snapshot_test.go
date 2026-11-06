package engine

import (
	"testing"
	"time"
)

func TestSnapshotFingerprintAndValidation(t *testing.T) {
	task := planTask("a")
	task.Parameters = map[string]string{"package": "one"}
	one, err := NewSnapshot([]Task{task}, nil)
	if err != nil {
		t.Fatal(err)
	}
	task.Parameters["package"] = "two"
	two, err := NewSnapshot([]Task{task}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if one.PlanHash == two.PlanHash {
		t.Fatal("plan changes not bound to snapshot")
	}
	now := time.Now().UTC()
	for _, result := range []TaskResult{
		{TaskID: "a", Status: Running},
		{TaskID: "a", Status: Success, FinishedAt: now},
		{TaskID: "a", Status: Pending},
		{TaskID: "a", Status: Failed, StartedAt: now, FinishedAt: now.Add(-time.Second)},
		{TaskID: "a", Status: Failed, FinishedAt: now, Attempts: -1},
	} {
		bad := one
		bad.Results = []TaskResult{result}
		if err := bad.Validate(); err == nil {
			t.Fatalf("invalid snapshot accepted: %+v", result)
		}
	}
}
