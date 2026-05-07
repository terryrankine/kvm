# Issue #013: actionSessions Map Race Condition

**Severity:** HIGH

## Issue

`actionSessions` (or equivalent active-session tracking map) is read and written from HTTP handler goroutines without a lock.

## Location

`web.go` — session management map.

## Fix

Protect with `sync.RWMutex`. Add `-race` test.

## Status

- [x] DONE — `webrtc.go`: added `actionSessionsMu sync.Mutex`. All reads/writes of `actionSessions` now hold the lock; capture `isFirst`/`isLast` booleans under the lock before calling side-effect functions outside it. `main.go` broadcaster callbacks also lock before reading `actionSessions`.
