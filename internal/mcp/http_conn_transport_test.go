package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type closeIdleSpy struct{ closed chan struct{} }

func (s *closeIdleSpy) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }
func (s *closeIdleSpy) CloseIdleConnections()                           { close(s.closed) }

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
