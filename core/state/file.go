// Package state stores versioned deployment checkpoints independently of UI.
package state

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"windeploy/core/engine"
)

const maxSnapshotBytes = 4 << 20

// FileStore uses a temporary file in the target directory, Sync, Close and Rename
// so failed writes do not truncate the previous checkpoint. Only one process may
// own a session path. The mutex serializes calls on this instance only.
// Rename durability/atomicity across power loss is filesystem dependent, especially
// on Windows; no power-loss guarantee is claimed. Directory must be trusted and
// protected by suitable Windows ACLs; Unix mode bits alone do not provide ACLs.
type FileStore struct {
	Path string
	mu   sync.Mutex
}

func (s *FileStore) Save(ctx context.Context, snapshot engine.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.Path == "" {
		return fmt.Errorf("snapshot path is required")
	}
	if err := snapshot.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if len(data) > maxSnapshotBytes {
		return fmt.Errorf("snapshot exceeds size limit")
	}
	dir := filepath.Dir(s.Path)
	file, err := os.CreateTemp(dir, ".windeploy-state-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	defer file.Close()
	if err = file.Chmod(0600); err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return os.Rename(temp, s.Path)
}

func (s *FileStore) Load(ctx context.Context) (engine.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return engine.Snapshot{}, err
	}
	file, err := os.Open(s.Path)
	if err != nil {
		return engine.Snapshot{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSnapshotBytes+1))
	if err != nil {
		return engine.Snapshot{}, err
	}
	if len(data) > maxSnapshotBytes {
		return engine.Snapshot{}, fmt.Errorf("snapshot exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var snapshot engine.Snapshot
	if err = decoder.Decode(&snapshot); err != nil {
		return engine.Snapshot{}, err
	}
	var extra interface{}
	if err = decoder.Decode(&extra); err != io.EOF {
		return engine.Snapshot{}, fmt.Errorf("trailing snapshot data")
	}
	if err = snapshot.Validate(); err != nil {
		return engine.Snapshot{}, err
	}
	if err = ctx.Err(); err != nil {
		return engine.Snapshot{}, err
	}
	return snapshot, nil
}
