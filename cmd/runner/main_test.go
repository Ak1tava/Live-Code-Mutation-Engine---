package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lcme/internal/sandbox"
)

func TestRunTestRejectsInvalidCompletionRecord(t *testing.T) {
	for _, record := range []string{
		``,
		`{"Name":"TestX"}`,
		`{"Name":"TestX","Failed":false}`,
		`{"Name":"TestX","Skipped":false}`,
		`{"Name":"TestOther","Failed":false,"Skipped":false}`,
		`{"Name":"TestX","Failed":true,"Skipped":false}`,
		`{"Name":"TestX","Failed":false,"Skipped":true}`,
		`{"Name":"TestX","Failed":false,"Skipped":false}{"Name":"TestX","Failed":false,"Skipped":false}`,
		`{"Name":"TestX","Failed":false,"Skipped":false}` + strings.Repeat(" ", 8192),
	} {
		label := record
		if len(label) > 100 {
			label = label[:100]
		}
		t.Run(label, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "fake-test")
			if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s' '"+record+"' >&3\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if got := runTest(dir, ".", bin, "TestX", time.Second); got.Status != sandbox.StatusFail {
				t.Fatalf("invalid record yielded %s", got.Status)
			}
		})
	}
}
