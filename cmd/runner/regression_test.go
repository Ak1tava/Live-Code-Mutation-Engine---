package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeRunnerProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestRunVariantPreservesSourceLinesAndCleansBinaries(t *testing.T) {
	source := "package sample\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {\n\tt.Error(\"first failure\")\n}\n\n\nfunc TestTwo(t *testing.T) {\n\n\tt.Error(\"second failure\")\n}\n"
	dir := writeRunnerProject(t, map[string]string{"go.mod": "module sample\n\ngo 1.24\n", "sample_test.go": source})
	bins := t.TempDir()
	result := runVariant(dir, "v", bins, time.Second)
	if result.BuildError != "" || len(result.Tests) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	for i, marker := range []string{"first failure", "second failure"} {
		line := strings.Count(source[:strings.Index(source, marker)], "\n") + 1
		if !strings.Contains(result.Tests[i].Output, fmt.Sprintf("sample_test.go:%d:", line)) {
			t.Errorf("reported position differs from original line %d: %s", line, result.Tests[i].Output)
		}
	}
	entries, err := os.ReadDir(bins)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("compiled variants retained after completion: %v", entries)
	}
}

func TestRunVariantBuildSelectedPackages(t *testing.T) {
	for _, tc := range []struct {
		name, pkg string
		extra     map[string]string
	}{
		{"nested module", "sample", map[string]string{"tools/go.mod": "module tools\n\ngo 1.24\n", "tools/tools_test.go": "package tools\nimport \"testing\"\nfunc TestTool(t *testing.T) { t.Fatal(\"nested module must be excluded\") }\n"}},
		{"excluded directory", "sample", map[string]string{"integration/check_test.go": "//go:build integration\n\npackage integration\nimport \"testing\"\nfunc TestExcluded(t *testing.T) {}\n"}},
		{"GOOS package name", "js", nil},
		{"external duplicate name", "sample", map[string]string{"external_test.go": "package sample_test\nimport \"testing\"\nvar again = TestValue\nfunc TestValue(t *testing.T) { if t.Name() != \"TestValue\" { t.Fatal(\"test identity changed\") } }\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{"go.mod": "module sample\n\ngo 1.24\n", "sample_test.go": "package " + tc.pkg + "\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n"}
			for k, v := range tc.extra {
				files[k] = v
			}
			result := runVariant(writeRunnerProject(t, files), "v", t.TempDir(), time.Second)
			want := 1
			if tc.name == "external duplicate name" {
				want = 2
			}
			if result.BuildError != "" || len(result.Tests) != want || !result.AllPassed(result.Keys()) {
				t.Fatalf("valid build-selected tests rejected: %+v", result)
			}
			if want == 2 && result.Tests[0].Key() == result.Tests[1].Key() {
				t.Fatal("internal and external tests must have distinct keys")
			}
		})
	}
}

func TestRunVariantFilesystemFailureIsNotBuildFailure(t *testing.T) {
	dir := writeRunnerProject(t, map[string]string{"go.mod": "module sample\n\ngo 1.24\n", "sample_test.go": "package sample\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n"})
	bins := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(bins, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	result := runVariant(dir, "v", bins, time.Second)
	if result.BuildError != "" || result.AllPassed(result.Keys()) {
		t.Fatalf("environment error mistaken for project compilation: %+v", result)
	}
}

func TestRunVariantRelativePath(t *testing.T) {
	t.Chdir(t.TempDir())
	dir := writeRunnerProject(t, map[string]string{"go.mod": "module sample\n\ngo 1.24\n", "sample_test.go": "package sample\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n"})
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, dir)
	if err != nil {
		t.Fatal(err)
	}
	result := runVariant(relative, "v", t.TempDir(), time.Second)
	if result.BuildError != "" || result.InfrastructureError != "" || len(result.Tests) != 1 || !result.AllPassed(result.Keys()) {
		t.Fatalf("relative batch path rejected: %+v", result)
	}
}

func TestRunVariantSeparatesDuplicateTestBodies(t *testing.T) {
	for _, failExternal := range []bool{false, true} {
		internal, external := "t.Fatal(\"internal failure\")", ""
		if failExternal {
			internal, external = "", "t.Fatal(\"external failure\")"
		}
		files := map[string]string{
			"go.mod":           "module sample\n\ngo 1.24\n",
			"internal_test.go": "package sample\nimport \"testing\"\nfunc TestValue(t *testing.T) { " + internal + " }\n",
			"external_test.go": "package sample_test\nimport \"testing\"\nvar ref = TestValue\nfunc TestValue(t *testing.T) { " + external + " }\n",
		}
		result := runVariant(writeRunnerProject(t, files), "v", t.TempDir(), time.Second)
		if result.BuildError != "" || len(result.Tests) != 2 {
			t.Fatalf("%+v", result)
		}
		if result.Passed(".:TestValue") != failExternal || result.Passed(".:TestValue (external)") == failExternal {
			t.Fatalf("duplicate bodies were not isolated: %+v", result)
		}
	}
}
