package state

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"windeploy/core/engine"
)

func testSnapshot(t *testing.T) engine.Snapshot {
	t.Helper()
	now := time.Now().UTC()
	s, err := engine.NewSnapshot([]engine.Task{{ID: "a", Name: "Task a", Category: "test", Status: engine.Pending, Parameters: map[string]string{"private": "never-store-me"}}}, []engine.TaskResult{{TaskID: "a", Status: engine.Success, StartedAt: now, FinishedAt: now, Attempts: 1}})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFileStoreRoundTripAndReplacement(t *testing.T) {
	dir := t.TempDir()
	store := &FileStore{Path: filepath.Join(dir, "session.json")}
	snapshot := testSnapshot(t)
	for i := 0; i < 2; i++ {
		snapshot.Results[0].RebootRequired = i == 1
		if err := store.Save(context.Background(), snapshot); err != nil {
			t.Fatal(err)
		}
		got, err := store.Load(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, snapshot) {
			t.Fatalf("%+v != %+v", got, snapshot)
		}
	}
	data, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "never-store-me") || strings.Contains(string(data), "parameters") {
		t.Fatal("parameters leaked")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary files leaked: %v", entries)
	}
}

func TestFileStoreRejectsCorruptData(t *testing.T) {
	store := &FileStore{Path: filepath.Join(t.TempDir(), "session.json")}
	for _, data := range []string{"{", `{"version":99}`, `{"version":1,"unknown":true}`, strings.Repeat(" ", maxSnapshotBytes+1)} {
		if err := os.WriteFile(store.Path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Load(context.Background()); err == nil {
			t.Fatal("corrupt state accepted")
		}
	}
	if err := store.Save(context.Background(), testSnapshot(t)); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(store.Path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString("{}")
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(context.Background()); err == nil {
		t.Fatal("trailing document accepted")
	}
}

func TestFileStoreFailedSavePreservesPrevious(t *testing.T) {
	store := &FileStore{Path: filepath.Join(t.TempDir(), "session.json")}
	snapshot := testSnapshot(t)
	if err := store.Save(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	bad := snapshot
	bad.Version = 99
	if err := store.Save(context.Background(), bad); err == nil {
		t.Fatal("bad version saved")
	}
	bad = snapshot
	bad.Results = append(bad.Results, bad.Results[0])
	if err := store.Save(context.Background(), bad); err == nil {
		t.Fatal("duplicate result saved")
	}
	bad = testSnapshot(t)
	bad.Results[0].Error = "password=unsafe"
	if err := store.Save(context.Background(), bad); err == nil {
		t.Fatal("raw diagnostic saved")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Save(ctx, snapshot); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
	if _, err := store.Load(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
	got, err := store.Load(context.Background())
	if err != nil || !reflect.DeepEqual(got, snapshot) {
		t.Fatalf("previous state lost: %+v %v", got, err)
	}
	missing := &FileStore{Path: filepath.Join(t.TempDir(), "missing", "session.json")}
	if err := missing.Save(context.Background(), snapshot); err == nil {
		t.Fatal("missing directory ignored")
	}
}
