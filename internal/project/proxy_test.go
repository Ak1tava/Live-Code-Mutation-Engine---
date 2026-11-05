package project

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestImportDNSRejectsNonPublicAndMixedAnswers(t *testing.T) {
	for _, raw := range []string{"0.1.2.3", "10.1.2.3", "100.64.1.2", "127.0.0.1", "169.254.169.254", "172.16.1.2", "192.168.1.1", "192.0.0.8", "192.0.2.5", "192.88.99.1", "198.18.0.1", "198.51.100.5", "203.0.113.5", "224.0.0.1", "240.1.2.3", "255.255.255.255", "::1", "::ffff:127.0.0.1", "2001:4860:4860::8888", "64:ff9b::a00:1"} {
		t.Run(raw, func(t *testing.T) {
			called := false
			_, err := dialGitHub(context.Background(), func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("140.82.114.3"), netip.MustParseAddr(raw)}, nil
			}, func(context.Context, string, string) (net.Conn, error) { called = true; return nil, nil })
			if err == nil || called {
				t.Fatalf("unsafe DNS %s accepted: dial=%v error=%v", raw, called, err)
			}
		})
	}
}

func TestImportDNSPinsCheckedIP(t *testing.T) {
	lookups := 0
	dialErr := errors.New("dial sentinel")
	_, err := dialGitHub(context.Background(), func(_ context.Context, network, host string) ([]netip.Addr, error) {
		lookups++
		if network != "ip4" || host != "github.com" {
			t.Fatalf("lookup %q %q", network, host)
		}
		return []netip.Addr{netip.MustParseAddr("140.82.114.3")}, nil
	}, func(_ context.Context, network, address string) (net.Conn, error) {
		if network != "tcp4" || address != "140.82.114.3:443" {
			t.Fatalf("unsafe dial %q %q", network, address)
		}
		return nil, dialErr
	})
	if !errors.Is(err, dialErr) || lookups != 1 {
		t.Fatalf("err=%v lookups=%d", err, lookups)
	}
}

func TestImportProxyDeniesOtherAuthorities(t *testing.T) {
	for _, request := range []string{
		"CONNECT 127.0.0.1:443 HTTP/1.1\r\nHost: 127.0.0.1:443\r\n\r\n",
		"CONNECT github.com:8443 HTTP/1.1\r\nHost: github.com:8443\r\n\r\n",
		"CONNECT evil.test:443 HTTP/1.1\r\nHost: evil.test:443\r\n\r\n",
		"CONNECT github.com:443 HTTP/1.1\r\nHost: github.com:443\r\nContent-Length: 4\r\n\r\nx",
		"GET https://github.com/a/b HTTP/1.1\r\nHost: github.com\r\n\r\n",
		"CONNECT github.com:443 HTTP/1.1\r\nHost: github.com:443\r\nX-Huge: " + strings.Repeat("x", 16384) + "\r\n\r\n",
	} {
		var dials atomic.Int32
		socket, stop, err := startImportProxy(context.Background(), func(context.Context) (net.Conn, error) { dials.Add(1); return nil, errors.New("not allowed") })
		if err != nil {
			t.Fatal(err)
		}
		conn, err := net.Dial("unix", socket)
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		_, _ = io.WriteString(conn, request)
		response, _ := io.ReadAll(conn)
		_ = conn.Close()
		stop()
		if !bytes.Contains(response, []byte("403 Forbidden")) || dials.Load() != 0 {
			t.Fatalf("request %.80q: response=%q dials=%d", request, response, dials.Load())
		}
	}
}

func TestImportProxyTunnelAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	socket, stop, err := startImportProxy(ctx, func(context.Context) (net.Conn, error) {
		a, b := net.Pipe()
		go func() { defer b.Close(); _, _ = io.Copy(b, b) }()
		return a, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = io.WriteString(conn, "CONNECT github.com:443 HTTP/1.1\r\nHost: github.com:443\r\n\r\n")
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("CONNECT: resp=%v err=%v", resp, err)
	}
	_, _ = io.WriteString(conn, "hello")
	got := make([]byte, 5)
	if _, err := io.ReadFull(reader, got); err != nil || string(got) != "hello" {
		t.Fatalf("tunnel=%q err=%v", got, err)
	}
	cancel()
	if _, err := reader.ReadByte(); err == nil {
		t.Fatal("cancelled tunnel stayed open")
	}
	stop()
	if _, err := os.Stat(socket); !os.IsNotExist(err) {
		t.Fatalf("socket not removed: %v", err)
	}
}

func TestImportTrafficBudgetSharedAcrossStreams(t *testing.T) {
	budget := &trafficBudget{}
	budget.remaining.Store(8)
	var out bytes.Buffer
	a, b := &budgetWriter{w: &out, budget: budget}, &budgetWriter{w: &out, budget: budget}
	if _, err := a.Write([]byte("12345")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("6789")); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("over-budget write: %v", err)
	}
	if out.String() != "12345" {
		t.Fatalf("over-budget bytes leaked: %q", &out)
	}
}

func TestImportStderrBounded(t *testing.T) {
	var stderr importStderr
	input := bytes.Repeat([]byte("x"), 1<<20)
	for range 5 {
		if n, err := stderr.Write(input); n != len(input) || err != nil {
			t.Fatalf("write=%d %v", n, err)
		}
	}
	// os/exec uses io.Copy; a promoted bytes.Buffer.ReadFrom must not bypass Write.
	if _, err := io.Copy(&stderr, io.LimitReader(zeroReader{}, 1<<20)); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 4096 {
		t.Fatalf("stderr retained %d bytes", stderr.Len())
	}
}

func TestImportProxyBoundsConcurrentConnections(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 4)
	socket, stop, err := startImportProxy(ctx, func(ctx context.Context) (net.Conn, error) {
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	for range 4 {
		conn, err := net.Dial("unix", socket)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_, _ = io.WriteString(conn, "CONNECT github.com:443 HTTP/1.1\r\nHost: github.com:443\r\n\r\n")
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("CONNECT not accepted")
		}
	}
	extra, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer extra.Close()
	_ = extra.SetDeadline(time.Now().Add(time.Second))
	if _, err := extra.Read(make([]byte, 1)); err == nil {
		t.Fatal("fifth concurrent connection accepted")
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("fifth connection was not rejected immediately")
	}
	cancel()
}
