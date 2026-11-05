// Package audit holds the session state machine, the report and the storage.
package audit

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"lcme/internal/hints"
	"lcme/internal/project"
	"lcme/internal/sandbox"
	"lcme/internal/selector"
)

// State is the lifecycle stage of a session.
type State string

const (
	StatePreparing State = "preparing"
	StateReady     State = "ready"
	StateRunning   State = "running"
	StateFinished  State = "finished"
	StateFailed    State = "failed"
)

const (
	// SessionDuration is the length of a session; the server owns the deadline.
	SessionDuration = 10 * time.Minute
	// MaxFileSize is the largest file a student may submit.
	MaxFileSize = 256 << 10
)

// Role identifies who a token belongs to.
type Role string

const (
	RoleAuditor Role = "auditor"
	RoleStudent Role = "student"
)

var (
	ErrBadState        = errors.New("audit: wrong session state")
	ErrReadOnly        = errors.New("audit: file is read-only")
	ErrUnknownFile     = errors.New("audit: unknown file")
	ErrExpired         = errors.New("audit: session time is over")
	ErrTooBig          = errors.New("audit: file is too big")
	ErrUnknownFlag     = errors.New("audit: unknown flag")
	ErrUnknownMutation = errors.New("audit: unknown mutation")
)

// Flag is a place in the code the student marked as suspicious.
type Flag struct {
	ID         string    `json:"id"`
	File       string    `json:"file"`
	Line       int       `json:"line"`
	Note       string    `json:"note"`
	At         time.Time `json:"at"`
	MutationID string    `json:"mutationId"`
	Accepted   *bool     `json:"accepted"`
}

// Run is one test run of the student's code.
type Run struct {
	At     time.Time             `json:"at"`
	Final  bool                  `json:"final"`
	Result sandbox.VariantResult `json:"result"`
}

// Event is a behavioural signal from the student's browser.
type Event struct {
	At   time.Time `json:"at"`
	Kind string    `json:"kind"` // "hidden" | "visible" | "paste"
	Size int       `json:"size"`
}

// Session is one audit exercise.
type Session struct {
	ID, AuditorToken, StudentToken string
	Source                         project.Source
	State                          State
	Error                          string
	Progress                       selector.Progress
	Plan                           *selector.Plan
	Hints                          []hints.Hint
	Files                          map[string][]byte // current non-test files of the student
	Tests                          map[string][]byte // *_test.go and other read-only files
	Mutated                        map[string][]byte // the task as issued (left side of the diff)
	CreatedAt, StartedAt           time.Time
	Deadline, FinishedAt           time.Time
	Flags                          []Flag
	Runs                           []Run
	Events                         []Event
}

func randomHex() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("audit: crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// New creates a session in the preparing state with fresh random tokens.
func New(src project.Source, now time.Time) *Session {
	return &Session{
		ID:           randomHex(),
		AuditorToken: randomHex(),
		StudentToken: randomHex(),
		Source:       src,
		State:        StatePreparing,
		CreatedAt:    now,
	}
}

func copyMap(m map[string][]byte) map[string][]byte {
	if m == nil {
		return nil
	}
	out := make(map[string][]byte, len(m))
	for k, v := range m {
		out[k] = append([]byte(nil), v...)
	}
	return out
}

// SetReady stores the prepared task. Files, Tests and Mutated are independent
// copies, so editing Files never changes Mutated.
func (s *Session) SetReady(_ *project.Project, plan *selector.Plan, h []hints.Hint) {
	s.Plan = plan
	s.Hints = h
	s.Mutated = copyMap(plan.Mutated)
	s.Files = map[string][]byte{}
	s.Tests = map[string][]byte{}
	for name, b := range s.Mutated {
		cp := append([]byte(nil), b...)
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			s.Files[name] = cp
		} else {
			s.Tests[name] = cp
		}
	}
	s.State = StateReady
}

// Fail marks the preparation as failed.
func (s *Session) Fail(msg string) {
	s.State = StateFailed
	s.Error = msg
}

// Start begins the countdown; the deadline is fixed here.
func (s *Session) Start(now time.Time) error {
	if s.State != StateReady {
		return ErrBadState
	}
	s.State = StateRunning
	s.StartedAt = now
	s.Deadline = now.Add(SessionDuration)
	return nil
}

func (s *Session) checkRunning(now time.Time) error {
	if s.State != StateRunning {
		return ErrBadState
	}
	if now.After(s.Deadline) {
		return ErrExpired
	}
	return nil
}

// UpdateFile replaces the content of one of the student's files.
func (s *Session) UpdateFile(path string, content []byte, now time.Time) error {
	if err := s.checkRunning(now); err != nil {
		return err
	}
	if _, ro := s.Tests[path]; ro {
		return ErrReadOnly
	}
	if _, ok := s.Files[path]; !ok {
		return ErrUnknownFile
	}
	if len(content) > MaxFileSize {
		return ErrTooBig
	}
	s.Files[path] = append([]byte(nil), content...)
	return nil
}

// AddFlag records a suspicious place.
func (s *Session) AddFlag(file string, line int, note string, now time.Time) (Flag, error) {
	if err := s.checkRunning(now); err != nil {
		return Flag{}, err
	}
	f := Flag{ID: fmt.Sprintf("f%d", len(s.Flags)+1), File: file, Line: line, Note: note, At: now}
	s.Flags = append(s.Flags, f)
	return f, nil
}

// ReviewFlag binds a flag to a mutation and records whether the explanation
// was accepted.
func (s *Session) ReviewFlag(flagID, mutationID string, accepted bool) error {
	idx := -1
	for i := range s.Flags {
		if s.Flags[i].ID == flagID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrUnknownFlag
	}
	known := false
	if s.Plan != nil {
		for _, m := range s.Plan.Mutations {
			if m.ID == mutationID {
				known = true
				break
			}
		}
	}
	if !known {
		return ErrUnknownMutation
	}
	s.Flags[idx].MutationID = mutationID
	s.Flags[idx].Accepted = &accepted
	return nil
}

// AddEvent appends a behavioural event. Validation is added later.
func (s *Session) AddEvent(kind string, size int, now time.Time) error {
	s.Events = append(s.Events, Event{At: now, Kind: kind, Size: size})
	return nil
}

// AddRun appends a test run.
func (s *Session) AddRun(r sandbox.VariantResult, final bool, now time.Time) {
	s.Runs = append(s.Runs, Run{At: now, Final: final, Result: r})
}

// Expired reports whether a running session is past its deadline.
func (s *Session) Expired(now time.Time) bool {
	return s.State == StateRunning && now.After(s.Deadline)
}

// Finish ends a running session; later calls keep the first FinishedAt.
func (s *Session) Finish(now time.Time) {
	if s.State != StateRunning {
		return
	}
	s.State = StateFinished
	s.FinishedAt = now
}

// RunFiles returns Files plus Tests; Tests override Files on conflict.
func (s *Session) RunFiles() map[string][]byte {
	out := make(map[string][]byte, len(s.Files)+len(s.Tests))
	for k, v := range s.Files {
		out[k] = v
	}
	for k, v := range s.Tests {
		out[k] = v
	}
	return out
}
