package mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type closeIdleSpy struct {
	closed chan struct{}
	calls  int
}

type cleanupCancellingTransport struct {
	requestStarted chan struct{}
	cleanupCalled  chan struct{}
}

func (t *cleanupCancellingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	close(t.requestStarted)
	<-t.cleanupCalled
	return nil, fmt.Errorf("legacy global cleanup cancelled request")
}

func (t *cleanupCancellingTransport) CloseIdleConnections() {
	close(t.cleanupCalled)
}

func (s *closeIdleSpy) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }
func (s *closeIdleSpy) CloseIdleConnections() {
	s.calls++
	if s.closed != nil {
		close(s.closed)
	}
}

// TestNewHTTPConnForTestOwnsTransport proves the test helper never falls back
// to http.DefaultTransport. Parallel httptest servers are allowed to close
// their own idle pools; a nil Client.Transport would make this client share
// process-global cleanup with unrelated tests.
func TestNewHTTPConnForTestOwnsTransport(t *testing.T) {
	conn := NewHTTPConnForTest("transport-owner", "http://example.test")
	if conn.client.Transport == nil {
		t.Fatal("NewHTTPConnForTest Client.Transport is nil; test client shares http.DefaultTransport")
	}
	if conn.client.Transport == http.DefaultTransport {
		t.Fatal("NewHTTPConnForTest Client.Transport is http.DefaultTransport; test client must own a clone")
	}
	got := conn.client.Transport.(*http.Transport)
	want := http.DefaultTransport.(*http.Transport)
	if got.MaxIdleConns != want.MaxIdleConns || got.MaxIdleConnsPerHost != want.MaxIdleConnsPerHost || got.IdleConnTimeout != want.IdleConnTimeout || got.TLSHandshakeTimeout != want.TLSHandshakeTimeout {
		t.Fatal("test transport clone does not retain default transport configuration")
	}
}

// TestHTTPConnTestTransportIsolatedFromDefaultCleanup proves the exact global
// coupling that made parallel httptest teardown unsafe. httptest.Server.Close
// calls CloseIdleConnections on http.DefaultTransport; nil clients delegate to
// that mutable global, while the test helper's cloned transport does not.
func TestHTTPConnTestTransportIsolatedFromDefaultCleanup(t *testing.T) {
	originalDefault := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalDefault })

	isolated := NewHTTPConnForTest("isolated", "http://example.test")
	legacy := NewHTTPConnForTest("legacy", "http://example.test")
	legacy.client.Transport = nil // the pre-fix construction contract
	spy := &closeIdleSpy{closed: make(chan struct{})}
	http.DefaultTransport = spy

	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	select {
	case <-spy.closed:
	default:
		t.Fatal("httptest.Server.Close did not call CloseIdleConnections on http.DefaultTransport")
	}
	if legacy.client.Transport != nil {
		t.Fatal("legacy nil-transport client no longer models default transport coupling")
	}
	if isolated.client.Transport == http.DefaultTransport {
		t.Fatal("isolated client shares the default transport cleanup target")
	}
}

// TestDialHTTPProductionTransportSurvivesUnrelatedGlobalCleanup captures the
// production path behind ClientManager. A production auth request is held in
// the default transport's dial gate while an unrelated httptest server closes
// the global idle pool. The connection under test must own a clone, so the
// request still reaches the server and returns the strict 401 sentinel.
//
// This test intentionally does not call t.Parallel because http.DefaultTransport
// is process-global test state and httptest.Server.Close acts on it.
func TestDialHTTPProductionTransportSurvivesUnrelatedGlobalCleanup(t *testing.T) {
	originalDefault := http.DefaultTransport
	base := originalDefault.(*http.Transport).Clone()
	dialStarted := make(chan struct{})
	releaseDial := make(chan struct{})
	base.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		close(dialStarted)
		select {
		case <-releaseDial:
			return (&net.Dialer{}).DialContext(ctx, network, address)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	http.DefaultTransport = base
	t.Cleanup(func() { http.DefaultTransport = originalDefault })

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer target.Close()

	conn, err := dialHTTP(ServerConfig{Name: "gated-auth", Transport: "http", URL: target.URL})
	if err != nil {
		t.Fatalf("dialHTTP: %v", err)
	}
	defer conn.Close()
	production := conn.(*httpConn)
	if production.client.Transport == nil || production.client.Transport == http.DefaultTransport {
		t.Fatal("dialHTTP shares http.DefaultTransport; unrelated cleanup can cancel its auth dial")
	}

	requestDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		requestDone <- conn.Initialize(ctx)
	}()
	select {
	case <-dialStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("production auth request never reached the dial gate")
	}

	unrelated := httptest.NewServer(http.NotFoundHandler())
	unrelated.Close()
	close(releaseDial)

	select {
	case err := <-requestDone:
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("Initialize error = %v, want strict ErrUnauthorized after unrelated cleanup", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("production auth request did not complete after releasing dial")
	}
}

// TestLegacyNilTransportRequestIsCancelledByUnrelatedGlobalCleanup models the
// historic production construction explicitly. A nil Client.Transport follows
// process-global http.DefaultTransport, so unrelated httptest teardown reaches
// that transport and cancels a held request before the server can return 401.
// The production test above proves the owned clone avoids this exact coupling.
//
// This test intentionally does not call t.Parallel because it changes
// process-global http.DefaultTransport.
func TestLegacyNilTransportRequestIsCancelledByUnrelatedGlobalCleanup(t *testing.T) {
	conn, err := dialHTTP(ServerConfig{Name: "legacy-auth", Transport: "http", URL: "http://example.test/mcp"})
	if err != nil {
		t.Fatalf("dialHTTP: %v", err)
	}
	legacy := conn.(*httpConn)
	legacy.client.Transport = nil // exact pre-#1190 production construction

	originalDefault := http.DefaultTransport
	cancelling := &cleanupCancellingTransport{
		requestStarted: make(chan struct{}),
		cleanupCalled:  make(chan struct{}),
	}
	http.DefaultTransport = cancelling
	t.Cleanup(func() { http.DefaultTransport = originalDefault })

	requestDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		requestDone <- conn.Initialize(ctx)
	}()
	select {
	case <-cancelling.requestStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("legacy nil-transport request never reached global transport")
	}

	unrelated := httptest.NewServer(http.NotFoundHandler())
	unrelated.Close()
	select {
	case <-cancelling.cleanupCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("unrelated httptest close did not reach the legacy global transport")
	}

	select {
	case err := <-requestDone:
		if err == nil {
			t.Fatal("legacy nil-transport request unexpectedly succeeded")
		}
		if errors.Is(err, ErrUnauthorized) {
			t.Fatalf("legacy nil-transport request returned ErrUnauthorized, want cleanup cancellation: %v", err)
		}
		if !strings.Contains(err.Error(), "legacy global cleanup cancelled request") {
			t.Fatalf("legacy nil-transport error = %v, want cleanup cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("legacy nil-transport request did not finish after global cleanup")
	}
}

// TestDialHTTPOwnsIndependentTransports prevents a future factory change from
// making two production connections share each other or the global transport.
func TestDialHTTPOwnsIndependentTransports(t *testing.T) {
	firstConn, err := dialHTTP(ServerConfig{Name: "first", Transport: "http", URL: "http://example.test/first"})
	if err != nil {
		t.Fatalf("first dialHTTP: %v", err)
	}
	defer firstConn.Close()
	secondConn, err := dialHTTP(ServerConfig{Name: "second", Transport: "http", URL: "http://example.test/second"})
	if err != nil {
		t.Fatalf("second dialHTTP: %v", err)
	}
	defer secondConn.Close()

	first := firstConn.(*httpConn).client.Transport
	second := secondConn.(*httpConn).client.Transport
	if first == nil || second == nil {
		t.Fatal("production dialHTTP returned a client with a nil transport")
	}
	if first == http.DefaultTransport || second == http.DefaultTransport {
		t.Fatal("production dialHTTP shares process-global http.DefaultTransport")
	}
	if first == second {
		t.Fatal("separate production connections share one idle-pool transport")
	}
}

// TestHTTPConnCloseClosesOnlyItsOwnedPool proves Close neither leaves owned
// idle connections behind nor reaches a sibling client's pool. It also
// verifies idempotence, which prevents a second Close from double-closing a
// transport resource during ClientManager shutdown.
func TestHTTPConnCloseClosesOnlyItsOwnedPool(t *testing.T) {
	firstTransport := &closeIdleSpy{closed: make(chan struct{})}
	secondTransport := &closeIdleSpy{closed: make(chan struct{})}
	first := &httpConn{client: &http.Client{Transport: firstTransport}}
	second := &httpConn{client: &http.Client{Transport: secondTransport}}

	if err := first.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("second first Close: %v", err)
	}
	if firstTransport.calls != 1 {
		t.Fatalf("first transport CloseIdleConnections calls = %d, want 1", firstTransport.calls)
	}
	if secondTransport.calls != 0 {
		t.Fatalf("second transport CloseIdleConnections calls = %d, want 0", secondTransport.calls)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if secondTransport.calls != 1 {
		t.Fatalf("second transport CloseIdleConnections calls = %d, want 1", secondTransport.calls)
	}
}
