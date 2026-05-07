# Issue #014: Goroutine Leak in WebSocket Handler

**Severity:** HIGH

## Issue

When a WebSocket client disconnects abnormally the read goroutine may not be cleaned up, leaking goroutines over time. On a resource-constrained device (128 MB RAM) this can exhaust goroutine stack space.

## Location

`web.go` — WebSocket read loop.

## Fix

Use a `context.Context` cancellation or `close` channel. Ensure the read goroutine exits on any write error and vice versa. Add a test that connects and abruptly closes the connection and asserts goroutine count returns to baseline.

## Status

- [x] NO LEAK — `handleWebRTCSignalWsMessages` uses `context.WithCancel`; the ping goroutine checks `runCtx.Err()` and returns on cancellation; `cancelRun()` is deferred so it fires when the function exits. The main read loop returns on any read error. No goroutine leak. No code change required.
