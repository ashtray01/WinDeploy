package reporting

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"windeploy/core/engine"
	"windeploy/core/inventory"
)

func sampleReport(t *testing.T) Report {
	t.Helper()
	tasks := []engine.Task{
		{ID: "sevenzip", Name: "7-Zip <script>", Category: "packages", Status: engine.Pending},
		{ID: "network", Name: "Сеть", Category: "diagnostics", Status: engine.Pending, Dependencies: []string{"sevenzip"}},
	}
	start := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	results := []engine.TaskResult{
		{TaskID: "sevenzip", Status: engine.Success, StartedAt: start, FinishedAt: start.Add(time.Minute), RebootRequired: true},
		{TaskID: "network", Status: engine.Failed, StartedAt: start.Add(time.Minute), FinishedAt: start.Add(2 * time.Minute), Error: "execute failed"},
	}
	report, err := Build("session-1", "office", "Офис & тест", inventory.SystemInfo{Hostname: "PC-01"}, tasks, results, start, start.Add(2*time.Minute), true, false)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestBuildSummary(t *testing.T) {
	report := sampleReport(t)
	if report.Summary.Total != 2 || report.Summary.Success != 1 || report.Summary.Failed != 1 || report.Summary.RebootRequired != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	if len(report.Tasks) != 2 || report.Tasks[1].Dependencies[0] != "sevenzip" {
		t.Fatalf("unexpected task preview: %+v", report.Tasks)
	}
	if len(report.Results) != 2 {
		t.Fatalf("unexpected results: %+v", report.Results)
	}
}

func TestWriteJSONAndHTML(t *testing.T) {
	report := sampleReport(t)
	var jsonBuffer bytes.Buffer
	if err := WriteJSON(&jsonBuffer, report); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(jsonBuffer.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	var htmlBuffer bytes.Buffer
	if err := WriteHTML(&htmlBuffer, report); err != nil {
		t.Fatal(err)
	}
	output := htmlBuffer.String()
	if !strings.Contains(output, "WinDeploy — отчёт") || !strings.Contains(output, "Офис &amp; тест") {
		preview := output
		if len(preview) > 500 {
			preview = preview[:500]
		}
		t.Fatalf("unexpected HTML:\n%s", preview)
	}
	if strings.Contains(output, "<script>") {
		t.Fatal("HTML did not escape a task name")
	}
}

func TestSaveRefusesOverwrite(t *testing.T) {
	report := sampleReport(t)
	directory := t.TempDir()
	jsonPath, htmlPath, err := Save(directory, report)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	for _, path := range []string{jsonPath, htmlPath} {
		if info, statErr := os.Stat(path); statErr != nil || info.Size() == 0 {
			t.Fatalf("report missing: %v %v", path, statErr)
		}
	}
	if _, _, err := Save(directory, report); err == nil {
		t.Fatal("second save with the same session must not overwrite")
	}
	if filepath.Dir(jsonPath) != directory {
		t.Fatalf("report written outside the requested directory: %s", jsonPath)
	}
}
