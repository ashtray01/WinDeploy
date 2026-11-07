package deploy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"windeploy/core/engine"
	"windeploy/core/inventory"
	"windeploy/core/trust"
)

// SessionVersion is the supported session file schema.
const SessionVersion = 1

// maxSessionBytes bounds a session document before any of it is loaded into memory.
const maxSessionBytes int64 = 16 << 20

// maxSessionTasks bounds a plan accepted from a session file.
const maxSessionTasks = 500

var sessionIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
var profileIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

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
	if !profileIDPattern.MatchString(s.ProfileID) || strings.TrimSpace(s.ProfileID) == "" {
		return fmt.Errorf("session has no valid profile")
	}
	if len([]rune(s.ProfileName)) > 200 || strings.ContainsAny(s.ProfileName, "\x00\r\n") {
		return fmt.Errorf("invalid session profile name")
	}
	if len(s.Tasks) == 0 || len(s.Tasks) > maxSessionTasks {
		return fmt.Errorf("session has no valid plan")
	}
	if len(s.BootID) > 128 {
		return fmt.Errorf("invalid session boot identity")
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
	// Trust overrides the owner/ACL verification used before a resumed, mutating
	// run. Nil uses the native Windows check. Tests inject this only to simulate a
	// rejected directory; production code must leave it nil.
	Trust func(context.Context, string, bool) error
}

func (s Store) path(id string) string {
	return filepath.Join(s.Dir, id+".json")
}

func (s Store) trust() func(context.Context, string, bool) error {
	if s.Trust != nil {
		return s.Trust
	}
	return trust.Check
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

// Load reads and validates one session. The file is bounded before it is read into
// memory, unexpected or trailing JSON is rejected, and the stored identity must
// match the requested session. Storage trust is checked separately by
// LoadForMutation before any system change.
func (s Store) Load(ctx context.Context, id string) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if strings.TrimSpace(s.Dir) == "" {
		return Session{}, fmt.Errorf("session directory is required")
	}
	if !validSessionID(id) {
		return Session{}, fmt.Errorf("invalid session ID")
	}
	data, err := readSessionFile(s.path(id), maxSessionBytes)
	if err != nil {
		return Session{}, err
	}
	session, err := decodeSession(data)
	if err != nil {
		return Session{}, err
	}
	if session.ID != id {
		return Session{}, fmt.Errorf("session identity does not match the requested session")
	}
	return session, nil
}

// LoadForMutation loads a session for a resumed, mutating run. The session
// directory and file must be trusted local storage owned by the current user, so a
// foreign or shared session cannot drive elevated changes. Trust failures are
// reported generically and never include ACL or file contents.
func (s Store) LoadForMutation(ctx context.Context, id string) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if strings.TrimSpace(s.Dir) == "" {
		return Session{}, fmt.Errorf("session directory is required")
	}
	if !validSessionID(id) {
		return Session{}, fmt.Errorf("invalid session ID")
	}
	check := s.trust()
	if err := check(ctx, s.Dir, true); err != nil {
		return Session{}, fmt.Errorf("session directory is not trusted")
	}
	if err := check(ctx, s.path(id), false); err != nil {
		return Session{}, fmt.Errorf("session file is not trusted")
	}
	return s.Load(ctx, id)
}

func validSessionID(id string) bool {
	return sessionIDPattern.MatchString(id) && filepath.Base(id) == id && !filepath.IsAbs(id)
}

// readSessionFile rejects links and non-regular files and never reads more than
// limit bytes, even when the file grows between stat and read.
func readSessionFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("session is unavailable")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("session file is too large")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("session is unavailable")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("session is unavailable")
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("session file is too large")
	}
	return data, nil
}

func decodeSession(data []byte) (Session, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var session Session
	if err := decoder.Decode(&session); err != nil {
		return Session{}, fmt.Errorf("session file is invalid")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return Session{}, fmt.Errorf("session file contains trailing data")
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
