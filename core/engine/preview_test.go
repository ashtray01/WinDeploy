package engine

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPreviewPlan(t *testing.T) {
	task := planTask("b", "a")
	task.Parameters = map[string]string{"password": "private-value"}
	task.Timeout = time.Minute
	task.Requirements = OSRequirements{Versions: []string{"Windows 7 SP1"}, Architectures: []string{"386"}}
	preview, err := PreviewPlan([]Task{task, planTask("a")})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Scope != "structure-and-dag-only" || len(preview.Tasks) != 2 || preview.Tasks[0].ID != "a" || preview.Tasks[1].Timeout != time.Minute || preview.Tasks[1].Name != task.Name || preview.Tasks[1].Category != task.Category {
		t.Fatalf("preview: %+v", preview)
	}
	data, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"parameters", "password", "private-value", "Success"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("unsafe preview: %s", data)
		}
	}
	preview.Tasks[1].Dependencies[0] = "changed"
	preview.Tasks[1].Requirements.Versions[0] = "changed"
	preview.Tasks[1].Requirements.Architectures[0] = "changed"
	if task.Dependencies[0] != "a" || task.Requirements.Versions[0] != "Windows 7 SP1" || task.Requirements.Architectures[0] != "386" {
		t.Fatal("preview aliases input")
	}
}

func TestPreviewPlanValidation(t *testing.T) {
	if preview, err := PreviewPlan(nil); err != nil || len(preview.Tasks) != 0 {
		t.Fatalf("empty plan: %+v %v", preview, err)
	}
	done := planTask("done")
	done.Status = Success
	for _, tasks := range [][]Task{{done}, {planTask("a", "missing")}, {planTask("a", "b"), planTask("b", "a")}} {
		if _, err := PreviewPlan(tasks); err == nil {
			t.Fatal("invalid plan accepted")
		}
	}
}
