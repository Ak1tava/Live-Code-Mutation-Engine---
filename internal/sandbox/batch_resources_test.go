package sandbox

import (
	"context"
	"fmt"
	"testing"
)

func TestDockerBatchReclaimsPackageBinaries(t *testing.T) {
	runner := dockerRunner(t)
	dir := t.TempDir()
	var ids []string
	for i := 0; i < 16; i++ {
		id := fmt.Sprintf("v%d", i)
		files := map[string][]byte{"go.mod": []byte("module fixture\n\ngo 1.24\n")}
		for pkg := 0; pkg < 3; pkg++ {
			folder := fmt.Sprintf("p%d/", pkg)
			files[folder+"value.go"] = []byte("package fixture\nimport (\"net/http\"; \"regexp\"; \"encoding/json\"; \"text/template\")\nvar _ = http.StatusOK\nvar _ = regexp.MustCompile\nvar _ = json.Marshal\nvar _ = template.New\nfunc Value() int { return 1 }\n")
			files[folder+"value_test.go"] = []byte("package fixture\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"value\") } }\n")
		}
		if err := WriteVariant(dir, id, files); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	results, err := runner.Run(context.Background(), Batch{Dir: dir, IDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 16 {
		t.Fatalf("got %d results", len(results))
	}
	for _, result := range results {
		if result.InfrastructureError != "" || result.BuildError != "" || len(result.Tests) != 3 || !result.AllPassed(result.Keys()) {
			t.Errorf("batch exhausted temporary storage: %+v", result)
		}
	}
}
