package nettest

import (
	"net"
	"testing"
	"time"
)

// TestDial passes only when the network is reachable; in the sandbox it must fail.
func TestDial(t *testing.T) {
	c, err := net.DialTimeout("tcp", "1.1.1.1:443", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}
