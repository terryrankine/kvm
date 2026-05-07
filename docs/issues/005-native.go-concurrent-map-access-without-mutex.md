# Issue #005: native.go Concurrent Map Access Without Mutex

**Severity:** CRITICAL

## Issue

`ctrlSocketConn` and `ongoingRequests` in `native.go` are accessed from multiple goroutines (the read loop, the write loop, and caller goroutines) without any synchronization. This is a data race that the Go race detector will flag and that can cause map corruption or nil-pointer panics in production.

## Location

`native.go` — global `ctrlSocketConn *net.Conn` and `ongoingRequests map[uint32]chan`.

## Fix

Protect both with a `sync.Mutex` (or replace map with `sync.Map`). Run `go test -race ./...` and add a concurrent stress test.

## Status

- [x] DONE — Added `requestsLock sync.RWMutex` to protect `ongoingRequests` separately from `lock`. `CallCtrlAction` now releases `lock` before blocking on the response channel so `handleCtrlClient` can acquire `requestsLock.RLock()` without deadlocking. `ctrlSocketConn` write in `handleCtrlClient` now under `lock.Lock/Unlock`. Response channels are buffered (size 1) to prevent `handleCtrlClient` from blocking on a cancelled caller.
