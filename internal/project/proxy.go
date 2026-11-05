package project

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type importDial func(context.Context) (net.Conn, error)

// This socket is mounted into exactly one container. No TCP listener is exposed
// on the host. At most 4 simultaneous/16 total tunnels share a 64 MiB budget.
func startImportProxy(ctx context.Context, dial importDial) (string, func(), error) {
	dir, err := os.MkdirTemp("", "lcme-import-proxy-")
	if err != nil {
		return "", nil, err
	}
	socket := filepath.Join(dir, "proxy.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	// The parent directory remains 0700; only the socket itself is bind-mounted.
	if err := os.Chmod(socket, 0o666); err != nil {
		_ = listener.Close()
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	stopAccept := context.AfterFunc(ctx, func() { _ = listener.Close() })
	var workers sync.WaitGroup
	workers.Add(1)
	slots := make(chan struct{}, 4)
	budget := &trafficBudget{}
	budget.remaining.Store(64 << 20)
	go func() {
		defer workers.Done()
		defer listener.Close()
		for attempts := 0; attempts < 16; attempts++ {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			select {
			case slots <- struct{}{}:
				workers.Add(1)
				go func() {
					defer workers.Done()
					defer func() { <-slots }()
					serveImportTunnel(ctx, conn, dial, budget)
				}()
			default:
				_ = conn.Close()
			}
		}
	}()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			_ = listener.Close()
			workers.Wait()
			stopAccept()
			_ = os.RemoveAll(dir)
		})
	}
	return socket, stop, nil
}

func serveImportTunnel(ctx context.Context, client net.Conn, dial importDial, budget *trafficBudget) {
	defer client.Close()
	stopClient := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stopClient()
	_ = client.SetDeadline(time.Now().Add(3 * time.Second))
	header := &io.LimitedReader{R: client, N: 8192}
	reader := bufio.NewReader(header)
	req, err := http.ReadRequest(reader)
	if err != nil || header.N <= 0 || req.Method != http.MethodConnect ||
		req.RequestURI != "github.com:443" || req.Host != "github.com:443" ||
		req.ContentLength > 0 || len(req.TransferEncoding) != 0 {
		_, _ = io.WriteString(client, "HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
		return
	}
	upstream, err := dial(ctx)
	if err != nil {
		_, _ = io.WriteString(client, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
		return
	}
	defer upstream.Close()
	stopUpstream := context.AfterFunc(ctx, func() { _ = upstream.Close() })
	defer stopUpstream()
	deadline := time.Now().Add(60 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	_ = client.SetDeadline(deadline)
	_ = upstream.SetDeadline(deadline)
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	// Preserve any bytes already buffered after CONNECT, now bounded by traffic.
	header.N = 64 << 20
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&budgetWriter{w: upstream, budget: budget}, reader)
		_ = upstream.Close()
		_ = client.Close()
		close(done)
	}()
	_, _ = io.Copy(&budgetWriter{w: client, budget: budget}, upstream)
	_ = upstream.Close()
	_ = client.Close()
	<-done
}

type trafficBudget struct{ remaining atomic.Int64 }

type budgetWriter struct {
	w      io.Writer
	budget *trafficBudget
}

func (w *budgetWriter) Write(p []byte) (int, error) {
	for {
		remaining := w.budget.remaining.Load()
		if int64(len(p)) > remaining {
			return 0, ErrTooLarge
		}
		if w.budget.remaining.CompareAndSwap(remaining, remaining-int64(len(p))) {
			return w.w.Write(p)
		}
	}
}

func publicGitHubDial(ctx context.Context) (net.Conn, error) {
	return dialGitHub(ctx, net.DefaultResolver.LookupNetIP, (&net.Dialer{Timeout: 5 * time.Second}).DialContext)
}

func dialGitHub(ctx context.Context,
	lookup func(context.Context, string, string) ([]netip.Addr, error),
	dial func(context.Context, string, string) (net.Conn, error),
) (net.Conn, error) {
	// MVP deliberately uses only IPv4. This also excludes IPv6 transition/NAT64
	// addresses whose apparent public range could route to private IPv4 targets.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ips, err := lookup(ctx, "ip4", "github.com")
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 || len(ips) > 16 {
		return nil, errors.New("no acceptable DNS answers")
	}
	for _, ip := range ips {
		if !publicImportIP(ip) {
			return nil, errors.New("non-public DNS answer")
		}
	}
	// Use the checked IP itself: a second hostname lookup would permit rebinding.
	return dial(ctx, "tcp4", net.JoinHostPort(ips[0].Unmap().String(), "443"))
}

func publicImportIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.Is4() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range excludedImportRanges {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

// Special-use, documentation, benchmarking, shared-address and reserved space.
var excludedImportRanges = func() []netip.Prefix {
	var prefixes []netip.Prefix
	for _, s := range []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24",
		"192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
	} {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			panic(fmt.Sprintf("invalid built-in network: %v", err))
		}
		prefixes = append(prefixes, p)
	}
	return prefixes
}()
