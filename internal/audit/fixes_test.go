package audit

import (
	"testing"
	"time"
)

func TestFixFlagAndEventLimits(t *testing.T) {
	s := runningSession(t)
	for range 100 {
		if _, err := s.AddFlag("textkit.go", 1, "note", t0.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddFlag("textkit.go", 1, "extra", t0.Add(time.Second)); err == nil {
		t.Error("unbounded flags")
	}
	s = runningSession(t)
	if _, err := s.AddFlag("textkit.go", 1, string(make([]byte, 4097)), t0.Add(time.Second)); err == nil {
		t.Error("unbounded note")
	}
	for range 1000 {
		if err := s.AddEvent("hidden", 0, t0.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AddEvent("hidden", 0, t0.Add(time.Second)); err == nil {
		t.Error("unbounded events")
	}
}
