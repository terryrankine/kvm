# Issue #015: Goroutine Leak in Native RPC Read Loop

**Severity:** HIGH

## Issue

The native RPC read goroutine in `native.go` may not exit when the underlying socket closes, leaking the goroutine and blocking callers waiting on response channels.

## Location

`native.go` — socket read loop.

## Fix

Signal all pending `ongoingRequests` channels with an error when the read loop exits. Add a test that closes the socket mid-request and asserts the caller unblocks with an error.

## Status

- [x] NO LEAK — `handleCtrlClient` read loop exits on error via `break` then function returns. Pending `CallCtrlAction` callers hit the `time.After(5 * time.Second)` timeout case which cleans up the map entry and returns an error. No goroutine escapes. The 5s wait on disconnect is a latency issue, not a leak. No code change required.
