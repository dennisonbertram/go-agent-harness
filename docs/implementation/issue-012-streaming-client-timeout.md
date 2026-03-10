# Issue #12: Remove Premature harnesscli Timeout for Streamed Runs

## Problem

Long-running harness runs were being terminated by `harnesscli` with the error
`scan event stream: context deadline exceeded`. This caused Terminal Bench runs
to fail with `unknown_agent_error` even when the harness was still making
progress.

## Root Cause

The `streamHTTPClient` was initialized as `&http.Client{}` — no client-level
timeout, but using `http.DefaultTransport` implicitly. The default transport has
`IdleConnTimeout: 90s`, which is designed for the short request/response cycle
pattern and is inappropriate for long-lived SSE streaming connections.

Additionally, the `http.DefaultTransport` has no explicit `ResponseHeaderTimeout`
setting (defaults to 0, which is fine), but having no explicit streaming-specific
transport made the behavior opaque and hard to reason about.

## Fix

Replaced the implicit default transport with an explicit `*http.Transport`
configured specifically for SSE streaming:

```go
streamHTTPClient = &http.Client{
    Transport: &http.Transport{
        IdleConnTimeout:       0,  // Never reap SSE connections
        ResponseHeaderTimeout: 0,  // Server controls event timing
        DisableKeepAlives:     false,
        Proxy:                 http.ProxyFromEnvironment,
        ForceAttemptHTTP2:     true,
        MaxIdleConns:          10,
        TLSHandshakeTimeout:   10 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
    },
}
```

The `requestHTTPClient` (used for `POST /v1/runs`) retains its `60s` timeout,
which is appropriate for short synchronous request/response calls.

## Files Changed

- `cmd/harnesscli/main.go` — streaming client now uses explicit transport
- `cmd/harnesscli/main_timeout_test.go` — new regression test file (4 tests)

## Tests Added

1. `TestStreamHTTPClientHasNoClientLevelTimeout` — verifies `streamHTTPClient.Timeout == 0`
2. `TestStreamHTTPClientUsesStreamingTransport` — verifies the transport is `*http.Transport`
   with `IdleConnTimeout: 0` and `ResponseHeaderTimeout: 0`
3. `TestStreamRunEventsCompletesAfterPauseLongerThanRequestTimeout` — verifies that a
   stream with an inter-event pause (150ms) longer than a hypothetical short timeout
   (50ms) completes successfully with the real `streamHTTPClient`; also validates the
   test logic by confirming a short-timeout client would fail
4. `TestRequestHTTPClientHas60SecondTimeout` — guard against accidentally removing the
   60s timeout from the request client

## TDD Process

1. Wrote failing tests in `main_timeout_test.go` — `TestStreamHTTPClientUsesStreamingTransport`
   failed because Transport was nil (using default)
2. Updated `streamHTTPClient` initialization in `main.go`
3. All 4 new tests pass; full suite passes (excluding pre-existing `demo-cli` build failure)
