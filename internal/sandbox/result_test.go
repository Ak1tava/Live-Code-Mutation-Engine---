package sandbox

import (
	"encoding/json"
	"testing"
)

func TestInfrastructureErrorCannotPass(t *testing.T) {
	var result VariantResult
	if err := json.Unmarshal([]byte(`{"id":"v","infrastructureError":"disk full","tests":[{"pkg":".","name":"TestX","status":"pass"}]}`), &result); err != nil {
		t.Fatal(err)
	}
	if result.Passed(".:TestX") || result.AllPassed([]string{".:TestX"}) || result.AllPassed(nil) {
		t.Fatal("infrastructure failure must not count as a successful test result")
	}
}
