package engine

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestTaskStatusValid(t *testing.T) {
	statuses := []TaskStatus{Pending, Running, Success, Failed, Skipped,
		AlreadyInstalled, NotApplicable, Cancelled, RebootRequired}
	seen := make(map[TaskStatus]bool)
	for _, status := range statuses {
		if !status.Valid() || seen[status] {
			t.Fatalf("invalid or duplicate status %q", status)
		}
		seen[status] = true
	}
	for _, status := range []TaskStatus{"", "success", "Unknown"} {
		if status.Valid() {
			t.Errorf("unrecognized status %q accepted", status)
		}
	}
}

func TestTaskValidate(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Task)
	}{
		{"empty ID", func(task *Task) { task.ID = " " }},
		{"empty name", func(task *Task) { task.Name = "\t" }},
		{"empty category", func(task *Task) { task.Category = "" }},
		{"negative timeout", func(task *Task) { task.Timeout = -time.Second }},
		{"zero status", func(task *Task) { task.Status = "" }},
		{"unknown status", func(task *Task) { task.Status = "unknown" }},
		{"empty dependency", func(task *Task) { task.Dependencies = []string{" "} }},
		{"self dependency", func(task *Task) { task.Dependencies = []string{task.ID} }},
		{"duplicate dependency", func(task *Task) { task.Dependencies = []string{"inventory", "inventory"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			task := Task{ID: "install", Name: "Install package", Category: "packages", Status: Pending}
			test.edit(&task)
			if err := task.Validate(); err == nil {
				t.Fatal("invalid task accepted")
			}
		})
	}
	for _, timeout := range []time.Duration{0, time.Minute} {
		task := Task{ID: "install", Name: "Install package", Category: "packages",
			Status: Pending, Timeout: timeout, Dependencies: []string{"inventory"}}
		if err := task.Validate(); err != nil {
			t.Fatalf("valid task rejected: %v", err)
		}
	}
}

func TestModelsJSONRoundTrip(t *testing.T) {
	task := Task{ID: "install", Name: "Установить пакет", Category: "packages",
		Status: Pending, Timeout: time.Minute, Dependencies: []string{"inventory"},
		Parameters:   map[string]string{"package": "example"},
		Requirements: OSRequirements{Versions: []string{"Windows 7 SP1"}, Architectures: []string{"386", "amd64"}}}
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	var decodedTask Task
	if err := json.Unmarshal(data, &decodedTask); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(task, decodedTask) {
		t.Fatalf("task changed after serialization: %#v", decodedTask)
	}
	start := time.Date(2026, time.November, 6, 10, 0, 0, 0, time.UTC)
	result := TaskResult{TaskID: task.ID, Status: Success, StartedAt: start,
		FinishedAt: start.Add(time.Second), RebootRequired: true}
	data, err = json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decodedResult TaskResult
	if err := json.Unmarshal(data, &decodedResult); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result, decodedResult) {
		t.Fatalf("result changed after serialization: %#v", decodedResult)
	}
}
