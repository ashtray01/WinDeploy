package engine

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func planTask(id string, dependencies ...string) Task {
	return Task{ID: id, Name: "Task " + id, Category: "test", Status: Pending,
		Dependencies: dependencies}
}

func TestOrderTasks(t *testing.T) {
	tests := []struct {
		name  string
		tasks []Task
		want  []string
	}{
		{"nil plan", nil, []string{}},
		{"empty plan", []Task{}, []string{}},
		{"single task", []Task{planTask("a")}, []string{"a"}},
		{"independent input order", []Task{planTask("z"), planTask("a"), planTask("m")}, []string{"z", "a", "m"}},
		{"reverse chain", []Task{planTask("c", "b"), planTask("b", "a"), planTask("a")}, []string{"a", "b", "c"}},
		{"diamond", []Task{planTask("d", "c", "b"), planTask("b", "a"), planTask("c", "a"), planTask("a")}, []string{"a", "b", "c", "d"}},
		{"newly ready earlier index", []Task{planTask("b", "a"), planTask("a"), planTask("z")}, []string{"a", "b", "z"}},
		{"disconnected graphs", []Task{planTask("b", "a"), planTask("d", "c"), planTask("c"), planTask("a")}, []string{"c", "d", "a", "b"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := OrderTasks(test.tasks)
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]string, len(got))
			for i, task := range got {
				ids[i] = task.ID
			}
			if !reflect.DeepEqual(ids, test.want) {
				t.Fatalf("order = %v, want %v", ids, test.want)
			}
		})
	}
}

func TestOrderTasksRejectsInvalidPlans(t *testing.T) {
	invalid := planTask("invalid")
	invalid.Name = ""
	tests := []struct {
		name  string
		tasks []Task
		want  string
	}{
		{"task validation", []Task{planTask("ok"), invalid}, `task at index 1 ("invalid"): task name is required`},
		{"duplicate IDs", []Task{planTask("a"), planTask("a")}, `duplicate task ID "a"`},
		{"unknown reference", []Task{planTask("a", "missing")}, `task "a": unknown dependency "missing"`},
		{"self dependency", []Task{planTask("a", "a")}, "task must not depend on itself"},
		{"duplicate dependency", []Task{planTask("a"), planTask("b", "a", "a")}, "duplicate dependency ID"},
		{"two node cycle", []Task{planTask("a", "b"), planTask("b", "a")}, `dependency cycle: unresolved tasks ["a" "b"]`},
		{"cycle and blocked task", []Task{planTask("ready"), planTask("a", "c"), planTask("b", "a"), planTask("c", "b"), planTask("d", "b")}, `dependency cycle: unresolved tasks ["a" "b" "c" "d"]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := OrderTasks(test.tasks)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want diagnostic containing %q", err, test.want)
			}
			if got != nil {
				t.Fatalf("invalid plan returned partial output: %v", got)
			}
		})
	}
}

func TestOrderTasksPreservesInput(t *testing.T) {
	for _, cyclic := range []bool{false, true} {
		name := "valid"
		if cyclic {
			name = "cyclic"
		}
		t.Run(name, func(t *testing.T) {
			tasks := []Task{planTask("b", "a"), planTask("a")}
			tasks[0].Parameters = map[string]string{"package": "example"}
			tasks[0].Requirements = OSRequirements{Versions: []string{"Windows 7 SP1"}, Architectures: []string{"386"}}
			if cyclic {
				tasks[1].Dependencies = []string{"b"}
			}
			// Use a deep copy so mutation of nested maps/slices is observable.
			data, err := json.Marshal(tasks)
			if err != nil {
				t.Fatal(err)
			}
			var before []Task
			if err := json.Unmarshal(data, &before); err != nil {
				t.Fatal(err)
			}
			ordered, err := OrderTasks(tasks)
			if (err != nil) != cyclic {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(tasks, before) {
				t.Fatal("input changed during ordering")
			}
			if cyclic {
				return
			}
			if !reflect.DeepEqual(ordered, []Task{before[1], before[0]}) {
				t.Fatal("task fields changed in ordered output")
			}
			ordered[0].ID = "changed"
			if !reflect.DeepEqual(tasks, before) {
				t.Fatal("output shares the input's outer slice")
			}
		})
	}
}
