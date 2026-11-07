package deploy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"windeploy/core/engine"
	"windeploy/core/inventory"
)

// SessionVersion is the supported session file schema.
const SessionVersion = 1

var sessionIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Session is a resumable deployment run. Parameters stay local; the file must be
// kept in a trusted per-user directory and never published.
type Session struct {
	Version        int                  `json:"version"`
	ID             string               `json:"id"`
	ProfileID      string               `json:"profile_id"`
	ProfileName    string               `json:"profile_name"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
	StartedAt      time.Time            `json:"started_at"`
	FinishedAt     time.Time            `json:"finished_at"`
	Tasks          []engine.Task        `json:"tasks"`
	Snapshot       engine.Snapshot      `json:"snapshot"`
	RebootRequired bool                 `json:"reboot_required"`
	Cancelled      bool                 `json:"cancelled"`
	Completed      bool                 `json:"completed"`
	ReportJSON     string               `json:"report_json,omitempty"`
	ReportHTML     string               `json:"report_html,omitempty"`
	ReportError    string               `json:"report_error,omitempty"`
	System         inventory.SystemInfo `json:"system"`
	BootID         string               `json:"boot_id,omitempty"`
	RebootBootIDs  map[string]string    `json:"reboot_boot_ids,omitempty"`
}

// Summary is a lightweight list entry.
type Summary struct {
	ID             string    `json:"id"`
	ProfileID      string    `json:"profile_id"`
	ProfileName    string    `json:"profile_name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Total          int       `json:"total"`
	Succeeded      int       `json:"succeeded"`
	Failed         int       `json:"failed"`
	Remaining      int       `json:"remaining"`
	Completed      bool      `json:"completed"`
	Cancelled      bool      `json:"cancelled"`
	RebootRequired bool      `json:"reboot_required"`
}

// Validate checks a session's identity and snapshot binding.
func (s Session) Validate() error {
	if s.Version != SessionVersion || !sessionIDPattern.MatchString(s.ID) {
		return fmt.Errorf("invalid session")
	}
	if strings.TrimSpace(s.ProfileID) == "" || len(s.Tasks) == 0 {
		return fmt.Errorf("session has no profile or tasks")
	}
	if s.UpdatedAt.Before(s.CreatedAt) {
		return fmt.Errorf("invalid session timestamps")
	}
	if err := s.Snapshot.ValidateForPlan(s.Tasks); err != nil {
		return fmt.Errorf("session snapshot does not match its plan")
	}
	return nil
}

// Summarize derives a list entry.
func (s Session) Summarize() Summary {
	summary := Summary{ID: s.ID, ProfileID: s.ProfileID, ProfileName: s.ProfileName, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt, Total: len(s.Tasks), Completed: s.Completed, Cancelled: s.Cancelled, RebootRequired: s.RebootRequired}
	seen := map[string]engine.TaskStatus{}
	for _, result := range s.Snapshot.Results {
		seen[result.TaskID] = result.Status
		switch result.Status {
		case engine.Success, engine.AlreadyInstalled, engine.NotApplicable:
			summary.Succeeded++
		case engine.Failed:
			summary.Failed++
		default:
			summary.Remaining++
		}
	}
	for _, task := range s.Tasks {
		if _, ok := seen[task.ID]; !ok {
			summary.Remaining++
		}
	}
	return summary
}

// Store persists sessions in one local directory.
type Store struct {
	Dir string
}

func (s Store) path(id string) string {
	return filepath.Join(s.Dir, id+".json")
}

// Save writes the session atomically and refuses invalid data.
func (s Store) Save(ctx context.Context, session Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := session.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(s.Dir) == "" {
		return fmt.Errorf("session directory is required")
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("session directory is unavailable")
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("session could not be encoded")
	}
	temp, err := os.CreateTemp(s.Dir, ".session-*")
	if err != nil {
		return fmt.Errorf("session file could not be created")
	}
	tempName := temp.Name()
	cleanup := func() { temp.Close(); os.Remove(tempName) }
	if err := temp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("session permissions are unavailable")
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("session could not be written")
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("session could not be flushed")
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempName)
		return fmt.Errorf("session could not be closed")
	}
	if err := os.Rename(tempName, s.path(session.ID)); err != nil {
		os.Remove(tempName)
		return fmt.Errorf("session could not be stored")
	}
	return nil
}

// Load reads and validates one session.
func (s Store) Load(ctx context.Context, id string) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if !sessionIDPattern.MatchString(id) {
		return Session{}, fmt.Errorf("invalid session ID")
	}
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return Session{}, fmt.Errorf("session is unavailable")
	}
	if len(data) > 16<<20 {
		return Session{}, fmt.Errorf("session file is too large")
	}
	var session Session
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&session); err != nil {
		return Session{}, fmt.Errorf("session file is invalid")
	}
	if err := session.Validate(); err != nil {
		return Session{}, err
	}
	return session, nil
}

// List returns summaries newest first, ignoring unreadable files.
func (s Store) List(ctx context.Context) ([]Summary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Summary{}, nil
		}
		return nil, fmt.Errorf("session directory is unavailable")
	}
	summaries := []Summary{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !sessionIDPattern.MatchString(id) {
			continue
		}
		session, err := s.Load(ctx, id)
		if err != nil {
			continue
		}
		summaries = append(summaries, session.Summarize())
		if len(summaries) > 500 {
			break
		}
	}
	sort.SliceStable(summaries, func(i, j int) bool { return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt) })
	return summaries, nil
}

// NewSessionID returns a sortable, local session identifier.
func NewSessionID() string {
	buffer := make([]byte, 4)
	_, _ = rand.Read(buffer)
	return "d" + time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(buffer)
}
