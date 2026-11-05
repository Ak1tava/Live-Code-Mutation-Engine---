package whoami

import (
	"os"
	"testing"
)

func TestUID(t *testing.T) {
	if os.Getuid() != 65534 {
		t.Fatalf("uid = %d, want 65534", os.Getuid())
	}
}
