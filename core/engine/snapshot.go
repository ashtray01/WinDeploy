package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const SnapshotVersion = 1

// Snapshot contains no task parameters or raw adapter diagnostics. PlanHash binds
// it to the exact Pending plan (including parameters) without storing that plan.
// It is not authentication: snapshots must be kept in a trusted local directory.
type Snapshot struct {
	Version  int          `json:"version"`
	PlanHash string       `json:"plan_hash"`
	Results  []TaskResult `json:"results"`
}

// NewSnapshot binds a set of results to a structurally valid Pending plan.
func NewSnapshot(tasks []Task, results []TaskResult) (Snapshot, error) {
	ordered, err := pendingPlan(tasks)
	if err != nil {
		return Snapshot{}, err
	}
	data, err := json.Marshal(ordered)
	if err != nil {
		return Snapshot{}, err
	}
	hash := sha256.Sum256(data)
	snapshot := Snapshot{Version: SnapshotVersion, PlanHash: hex.EncodeToString(hash[:]), Results: append([]TaskResult{}, results...)}
	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

// Validate checks the file format, not the binding or dependency consistency.
// Call ValidateForPlan before trusting results for execution (added by resume).
func (s Snapshot) Validate() error {
	if s.Version != SnapshotVersion {
		return fmt.Errorf("unsupported snapshot version")
	}
	hash, err := hex.DecodeString(s.PlanHash)
	if err != nil || len(hash) != sha256.Size {
		return fmt.Errorf("invalid snapshot plan hash")
	}
	seen := make(map[string]bool, len(s.Results))
	for _, r := range s.Results {
		if strings.TrimSpace(r.TaskID) == "" || seen[r.TaskID] {
			return fmt.Errorf("invalid or duplicate snapshot task ID")
		}
		seen[r.TaskID] = true
		if !r.Status.Valid() || r.Status == Pending || r.Attempts < 0 {
			return fmt.Errorf("invalid snapshot task state")
		}
		if r.Status == RebootRequired && (!r.RebootRequired || r.StartedAt.IsZero()) {
			return fmt.Errorf("invalid pending reboot evidence")
		}
		switch r.Error {
		case "", "precheck failed", "execute failed", "verify failed", "precheck returned an invalid outcome", "task deadline exceeded", "plan cancelled", "dependency did not succeed":
		default:
			return fmt.Errorf("snapshot contains an unsupported diagnostic")
		}
		if r.Status == Running {
			if r.StartedAt.IsZero() || !r.FinishedAt.IsZero() {
				return fmt.Errorf("invalid running task timestamps")
			}
		} else if r.FinishedAt.IsZero() || (!r.StartedAt.IsZero() && r.FinishedAt.Before(r.StartedAt)) {
			return fmt.Errorf("invalid completed task timestamps")
		}
		if (r.Status == Success || r.Status == AlreadyInstalled) && r.StartedAt.IsZero() {
			return fmt.Errorf("successful task has no start time")
		}
	}
	return nil
}
