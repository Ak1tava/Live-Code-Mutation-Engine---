package api

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"lcme/internal/audit"
	"lcme/internal/hints"
	"lcme/internal/mutate"
	"lcme/internal/sandbox"
)

type StudentView struct {
	State     audit.State       `json:"state"`
	ServerNow time.Time         `json:"serverNow"`
	Deadline  *time.Time        `json:"deadline,omitempty"`
	Running   bool              `json:"running"`
	Finishing bool              `json:"finishing"`
	Files     map[string]string `json:"files"`
	ReadOnly  []string          `json:"readOnly"`
	Runs      []audit.Run       `json:"runs"`
	Flags     []studentFlag     `json:"flags"`
	Report    *studentReport    `json:"report,omitempty"`
}

type studentFlag struct {
	ID   string `json:"id"`
	File string `json:"file"`
	Line int    `json:"line"`
	Note string `json:"note"`
}

type studentReport struct {
	Restored    bool `json:"restored"`
	DurationSec int  `json:"durationSec"`
}

type AuditorView struct {
	StudentView
	Progress     progressView      `json:"progress"`
	Error        string            `json:"error"`
	StudentURL   string            `json:"studentUrl"`
	Mutations    []mutate.Mutation `json:"mutations"`
	KillingTests [][]string        `json:"killingTests"`
	Hints        []hints.Hint      `json:"hints"`
	Mutated      map[string]string `json:"mutated"`
	Flags        []audit.Flag      `json:"flags"`
	Events       []audit.Event     `json:"events"`
	Stats        statsView         `json:"stats"`
	Report       *audit.Report     `json:"report,omitempty"`
}

type progressView struct {
	Stage string `json:"stage"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

type statsView struct {
	Candidates     int `json:"candidates"`
	Killed         int `json:"killed"`
	Survived       int `json:"survived"`
	BuildErrors    int `json:"buildErrors"`
	TriplesChecked int `json:"triplesChecked"`
}

func studentFlagOf(f audit.Flag) studentFlag {
	return studentFlag{ID: f.ID, File: f.File, Line: f.Line, Note: f.Note}
}

func addText(dst map[string]string, src map[string][]byte) {
	for path, content := range src {
		dst[path] = string(content)
	}
}

// newViews copies all view data that may be used after Store.View returns.
func (s *Server) newViews(v *audit.Session, rt *sessionRuntime, auditor bool) any {
	now := s.d.Now()
	finishing := v.State == audit.StateRunning && (!v.FinishRequestedAt.IsZero() || v.Expired(now))
	student := StudentView{
		State: v.State, ServerNow: now, Running: rt.active || finishing, Finishing: finishing,
		Files: make(map[string]string, len(v.Files)+len(v.Tests)), ReadOnly: make([]string, 0, len(v.Tests)),
		Runs: append([]audit.Run{}, v.Runs...), Flags: make([]studentFlag, 0, len(v.Flags)),
	}
	if !v.Deadline.IsZero() {
		deadline := v.Deadline
		student.Deadline = &deadline
	}
	addText(student.Files, v.Files)
	addText(student.Files, v.Tests)
	for name := range v.Tests {
		student.ReadOnly = append(student.ReadOnly, name)
	}
	sort.Strings(student.ReadOnly)
	for i := range student.Runs {
		res := &student.Runs[i].Result
		res.Tests = append([]sandbox.TestResult{}, res.Tests...)
	}
	for _, flag := range v.Flags {
		student.Flags = append(student.Flags, studentFlagOf(flag))
	}
	var report *audit.Report
	if v.State == audit.StateFinished {
		result := audit.BuildReport(v)
		report = &result
		student.Report = &studentReport{Restored: result.Restored, DurationSec: result.DurationSec}
	}
	if !auditor {
		return student
	}

	view := AuditorView{
		StudentView: student, Progress: progressView(v.Progress), Error: v.Error,
		Mutations: []mutate.Mutation{}, KillingTests: [][]string{}, Hints: append([]hints.Hint{}, v.Hints...),
		Mutated: make(map[string]string, len(v.Mutated)), Flags: append([]audit.Flag{}, v.Flags...),
		Events: append([]audit.Event{}, v.Events...), Report: report,
	}
	for i := range view.Flags {
		if accepted := view.Flags[i].Accepted; accepted != nil {
			value := *accepted
			view.Flags[i].Accepted = &value
		}
	}
	if v.State == audit.StateReady || v.State == audit.StateRunning || v.State == audit.StateFinished {
		view.StudentURL = strings.TrimRight(s.d.PublicURL, "/") + "/s/" + v.StudentToken
	}
	addText(view.Mutated, v.Mutated)
	if p := v.Plan; p != nil {
		view.Mutations = append(view.Mutations, p.Mutations...)
		for _, tests := range p.KillingTests {
			view.KillingTests = append(view.KillingTests, append([]string{}, tests...))
		}
		view.Stats = statsView(p.Stats)
	}
	return view
}

func (s *Server) getView(w http.ResponseWriter, _ *http.Request, id string, role audit.Role) {
	rt := s.runtime(id)
	rt.mu.Lock()
	var view any
	s.d.Store.View(id, func(v *audit.Session) {
		view = s.newViews(v, rt, role == audit.RoleAuditor)
	})
	rt.mu.Unlock()
	writeJSON(w, http.StatusOK, view)
}
