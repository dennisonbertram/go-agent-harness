package mcp

import (
	"net/http"
	"time"
)

// HTTPConnForTest is an alias for httpConn exported only for tests.
type HTTPConnForTest = httpConn

// NewHTTPConnForTest creates an httpConn for testing with the given name and URL.
func NewHTTPConnForTest(name, url string) *httpConn {
	return &httpConn{
		name:     name,
		endpoint: url,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// NegotiatedVersion returns the negotiated protocol version for testing.
func (c *httpConn) NegotiatedVersion() string {
	return c.negotiatedVersion
}
