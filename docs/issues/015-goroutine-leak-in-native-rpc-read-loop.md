# Issue #015: Goroutine Leak in Native RPC Read Loop

**Severity:** HIGH

## Issue

The native RPC read goroutine in `native.go` may not exit when the underlying socket closes, leaking the goroutine and blocking callers waiting on response channels.

## Location

`native.go` — socket read loop.

## Fix

Signal all pending `ongoingRequests` channels with an error when the read loop exits. Add a test that closes the socket mid-request and asserts the caller unblocks with an error.

## Status

- [ ] TODO
