package sandbox

import (
	"testing"
)

// The runner must establish that the requested test completed successfully;
// neither a zero process exit nor text printed by the project establishes it.
func TestLocalRequiresTestCompletion(t *testing.T) {
	checkTestCompletion(t, runOne)
}

func TestDockerRequiresTestCompletion(t *testing.T) {
	dockerRunner(t)
	checkTestCompletion(t, dockerRunOne)
}

func checkTestCompletion(t *testing.T, run func(*testing.T, Batch) VariantResult) {
	t.Helper()
	cases := []struct {
		name, tests, source string
	}{
		{
			name: "init exits before tests",
			tests: `import "testing"
func TestX(t *testing.T) { t.Fatal("must run") }`,
			source: `import "os"
func init() { os.Exit(0) }`,
		},
		{
			name: "TestMain omits m.Run",
			tests: `import ("os"; "testing")
func TestMain(m *testing.M) { os.Exit(0) }
func TestX(t *testing.T) { t.Fatal("must run") }`,
		},
		{
			name: "TestMain changes run filter",
			tests: `import ("flag"; "testing")
func TestMain(m *testing.M) { flag.Parse(); flag.Set("test.run", "^Nothing$"); m.Run() }
func TestX(t *testing.T) { t.Fatal("must run") }`,
		},
		{
			name: "TestMain masks failure",
			tests: `import ("os"; "testing")
func TestMain(m *testing.M) { m.Run(); os.Exit(0) }
func TestX(t *testing.T) { t.Fatal("failed") }`,
		},
		{
			name: "skipped test",
			tests: `import "testing"
func TestX(t *testing.T) { t.Skip("no result") }`,
		},
		{
			name: "test exits without cleanup",
			tests: `import ("os"; "testing")
func TestX(t *testing.T) { os.Exit(0) }`,
		},
		{
			name: "forged stdout and test listing",
			tests: `import "testing"
func TestX(t *testing.T) { t.Fatal("must run") }`,
			source: `import ("fmt"; "os")
func init() {
 fmt.Println("TestX\nTestInvented\n=== RUN   TestX\n--- PASS: TestX (0.00s)\nPASS")
 fmt.Println("{\"Action\":\"pass\",\"Test\":\"TestX\"}")
 os.Exit(0)
}`,
		},
		{
			name: "cleanup failure masked by TestMain",
			tests: `import ("os"; "testing")
func TestMain(m *testing.M) { m.Run(); os.Exit(0) }
func TestX(t *testing.T) { t.Cleanup(func() { t.Error("late failure") }) }`,
		},
		{
			name: "subtest failure masked by TestMain",
			tests: `import ("os"; "testing")
func TestMain(m *testing.M) { m.Run(); os.Exit(0) }
func TestX(t *testing.T) { t.Run("child", func(t *testing.T) { t.Parallel(); t.Error("failed") }) }`,
		},
		{
			name: "TestMain reruns and masks later failure",
			tests: `import ("os"; "testing")
var runs int
func TestMain(m *testing.M) { m.Run(); m.Run(); os.Exit(0) }
func TestX(t *testing.T) { runs++; if runs > 1 { t.Fatal("second run failed") } }`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := run(t, variant(t, tinyProject(tc.tests, tc.source)))
			if r.BuildError != "" || len(r.Tests) != 1 || r.Tests[0].Key() != ".:TestX" {
				t.Fatalf("expected the actual compiled test, got %+v", r)
			}
			if r.Tests[0].Status != StatusFail {
				t.Fatalf("unfinished/skipped/failed test counted as %s: %+v", r.Tests[0].Status, r)
			}
		})
	}
}

func TestLocalCompletionSupportsGoTestLifecycle(t *testing.T) {
	files := tinyProject(`import ("os"; "testing")
func TestMain(m *testing.M) { os.Exit(m.Run()) }
func TestX(t *testing.T) {
 t.Parallel()
 t.Cleanup(func() { t.Log("cleanup ran") })
 t.Run("child", func(t *testing.T) { t.Parallel() })
}
func TestUnnamed(*testing.T) {}
`, "")
	files["external_test.go"] = []byte(`package tiny_test
import . "testing"
func TestExternal(t *T) { t.Run("child", func(t *T) {}) }
`)
	files["ignored_test.go"] = []byte(`//go:build lcme_test_excluded

package tiny
import "testing"
func TestIgnored(t *testing.T) { t.Fatal("excluded file executed") }
`)
	r := runOne(t, variant(t, files))
	if r.BuildError != "" || len(r.Tests) != 3 {
		t.Fatalf("%+v", r)
	}
	for _, key := range []string{".:TestX", ".:TestUnnamed", ".:TestExternal"} {
		if !r.Passed(key) {
			t.Errorf("missing completed test %s: %+v", key, r)
		}
	}
}
