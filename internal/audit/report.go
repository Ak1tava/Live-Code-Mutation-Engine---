package audit

import "lcme/internal/mutate"

// MutationReport is the outcome for one planted mutation.
type MutationReport struct {
	ID            string      `json:"id"`
	File          string      `json:"file"`
	Line          int         `json:"line"`
	Kind          mutate.Kind `json:"kind"`
	Orig          string      `json:"orig"`
	Repl          string      `json:"repl"`
	Detected      bool        `json:"detected"`
	Explained     bool        `json:"explained"`
	Fixed         bool        `json:"fixed"`
	FixedAfterSec int         `json:"fixedAfterSec"` // -1 if never fixed
}

// Report is the auditor's summary of a session.
type Report struct {
	VerificationError string           `json:"verificationError,omitempty"`
	Mutations         []MutationReport `json:"mutations"`
	Restored          bool             `json:"restored"`
	HiddenCount       int              `json:"hiddenCount"`
	PasteCount        int              `json:"pasteCount"`
	PasteChars        int              `json:"pasteChars"`
	DurationSec       int              `json:"durationSec"`
}

// BuildReport computes the report. Fixed is judged by the tests of the last
// final run, never by comparing text.
func BuildReport(s *Session) Report {
	r := Report{Mutations: []MutationReport{}}

	var final *Run
	for i := range s.Runs {
		if s.Runs[i].Final {
			final = &s.Runs[i]
		}
	}
	if final != nil {
		r.VerificationError = final.Result.InfrastructureError
	}

	if s.Plan != nil {
		for i, m := range s.Plan.Mutations {
			mr := MutationReport{
				ID: m.ID, File: m.File, Line: m.Line, Kind: m.Kind,
				Orig: m.Orig, Repl: m.Repl, FixedAfterSec: -1,
			}
			for _, f := range s.Flags {
				if f.MutationID != m.ID {
					continue
				}
				mr.Detected = true
				if f.Accepted != nil && *f.Accepted {
					mr.Explained = true
				}
			}
			if i < len(s.Plan.KillingTests) {
				keys := s.Plan.KillingTests[i]
				if final != nil {
					mr.Fixed = final.Result.AllPassed(keys)
				}
				for _, run := range s.Runs {
					if run.Result.AllPassed(keys) {
						mr.FixedAfterSec = int(run.At.Sub(s.StartedAt).Seconds())
						break
					}
				}
			}
			r.Mutations = append(r.Mutations, mr)
		}
		if final != nil {
			r.Restored = final.Result.BuildError == "" && final.Result.AllPassed(s.Plan.BaselineTests)
		}
	}

	for _, e := range s.Events {
		switch e.Kind {
		case "hidden":
			r.HiddenCount++
		case "paste":
			r.PasteCount++
			r.PasteChars += e.Size
		}
	}
	if !s.FinishedAt.IsZero() && !s.StartedAt.IsZero() {
		r.DurationSec = int(s.FinishedAt.Sub(s.StartedAt).Seconds())
	}
	return r
}
