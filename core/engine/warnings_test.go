package engine

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type warningAdapter struct {
	fakeAdapter
	warnings map[string][]string
}

func (a *warningAdapter) TaskWarnings(task Task) []string { return a.warnings[task.ID] }

func TestWarningsAreCollectedAndSanitized(t *testing.T) {
	long := strings.Repeat("я", WarningTextLimit+10)
	a := &warningAdapter{warnings: map[string][]string{
		"a": {"  Ярлык не подтверждён  ", "Ярлык не подтверждён", "строка\r\nперенос", long},
	}}
	results, err := Run(context.Background(), []Task{planTask("a")}, a)
	if err != nil {
		t.Fatal(err)
	}
	warnings := results[0].Warnings
	if len(warnings) != 4 {
		t.Fatalf("unexpected warning count: %d", len(warnings))
	}
	if warnings[0] != "Ярлык не подтверждён" || warnings[1] != "строка  перенос" {
		t.Fatalf("warnings not normalized: %q %q", warnings[0], warnings[1])
	}
	if len([]rune(warnings[2])) != WarningTextLimit || len([]rune(warnings[3])) != WarningTextLimit {
		t.Fatalf("warnings not bounded: %d %d", len([]rune(warnings[2])), len([]rune(warnings[3])))
	}
	// Truncated warnings are bounded; identical notices are de-duplicated.
	other := strings.Repeat("ж", WarningTextLimit+5)
	if got := SanitizeWarnings([]string{long, other}); len(got) != 2 || len([]rune(got[1])) != WarningTextLimit {
		t.Fatalf("long warnings not bounded: %d", len(got))
	}
	if got := SanitizeWarnings([]string{"повтор", "повтор"}); len(got) != 1 {
		t.Fatalf("duplicate warnings kept: %d", len(got))
	}
}

func TestWarningsAreBoundedAndPersisted(t *testing.T) {
	many := make([]string, 0, MaxWarnings+3)
	for i := 0; i < MaxWarnings+3; i++ {
		many = append(many, "предупреждение "+string(rune('a'+i)))
	}
	a := &warningAdapter{warnings: map[string][]string{"a": many}}
	store := &memoryStore{}
	results, err := RunWithOptions(context.Background(), []Task{planTask("a")}, a, RunOptions{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if len(results[0].Warnings) != MaxWarnings {
		t.Fatalf("warnings not bounded: %d", len(results[0].Warnings))
	}
	snapshot := store.snapshots[len(store.snapshots)-1]
	if !reflect.DeepEqual(snapshot.Results[0].Warnings, results[0].Warnings) {
		t.Fatalf("snapshot warnings differ: %q", snapshot.Results[0].Warnings)
	}
}

func TestWarningsKeptOnFailureAndReboot(t *testing.T) {
	a := &warningAdapter{fakeAdapter: fakeAdapter{verify: func(context.Context, Task) error { return ErrRebootPending }},
		warnings: map[string][]string{"a": {"Принтер по умолчанию не изменён"}}}
	results, err := Run(context.Background(), []Task{planTask("a")}, a)
	if err != nil || results[0].Status != RebootRequired {
		t.Fatalf("unexpected reboot result: %+v %v", results, err)
	}
	if len(results[0].Warnings) != 1 {
		t.Fatalf("warnings lost on reboot: %q", results[0].Warnings)
	}
	// A fresh adapter after reboot reports no warnings; the persisted notice stays.
	a2 := &warningAdapter{}
	prior, err := NewSnapshot([]Task{planTask("a")}, results)
	if err != nil {
		t.Fatal(err)
	}
	results, err = Resume(context.Background(), []Task{planTask("a")}, a2, prior, ResumeOptions{VerifyReboot: true})
	if err != nil || results[0].Status != Success {
		t.Fatalf("resume failed: %+v %v", results, err)
	}
	if len(results[0].Warnings) != 1 {
		t.Fatalf("warnings lost after reboot verification: %q", results[0].Warnings)
	}
}

func TestSnapshotRejectsInvalidWarnings(t *testing.T) {
	tasks := []Task{planTask("a")}
	base := completedResult("a", Success)
	for _, warnings := range [][]string{{" "}, {"line\nbreak"}, {strings.Repeat("x", WarningTextLimit+1)}, make([]string, MaxWarnings+1)} {
		result := base
		result.Warnings = warnings
		snapshot, err := NewSnapshot(tasks, []TaskResult{result})
		if err == nil {
			t.Fatalf("invalid warnings accepted: %q (%+v)", warnings, snapshot)
		}
	}
}

func TestSanitizeWarningsIgnoresEmpty(t *testing.T) {
	if got := SanitizeWarnings(nil); len(got) != 0 {
		t.Fatalf("unexpected warnings: %q", got)
	}
	if got := SanitizeWarnings([]string{"", "  "}); len(got) != 0 {
		t.Fatalf("blank warnings kept: %q", got)
	}
}
