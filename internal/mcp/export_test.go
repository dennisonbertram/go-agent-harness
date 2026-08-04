package mcp

import (
	"net/http"
	"time"
)

// HTTPConnForTest is an alias for httpConn exported only for tests.
type HTTPConnForTest = httpConn

// newHTTPTestClient returns a client with an independent idle-connection pool.
// A nil Transport would use process-global http.DefaultTransport, which lets
// an unrelated parallel httptest.Server teardown interrupt this fixture's
// request on Go versions that close default idle connections.
func newHTTPTestClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	return &http.Client{Transport: transport, Timeout: 30 * time.Second}
}

// NewHTTPConnForTest creates an httpConn for testing with the given name and URL.
func NewHTTPConnForTest(name, url string) *httpConn {
	return &httpConn{
		name:     name,
		endpoint: url,
		client:   newHTTPTestClient(),
	}
}

// NegotiatedVersion returns the negotiated protocol version for testing.
func (c *httpConn) NegotiatedVersion() string {
	return c.negotiatedVersion
}

// DialHTTPForTest exposes dialHTTP for white-box testing of URL validation.
func DialHTTPForTest(cfg ServerConfig) (Conn, error) {
	return dialHTTP(cfg)
}
