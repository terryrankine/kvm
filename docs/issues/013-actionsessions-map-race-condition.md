# Issue #013: actionSessions Map Race Condition

**Severity:** HIGH

## Issue

`actionSessions` (or equivalent active-session tracking map) is read and written from HTTP handler goroutines without a lock.

## Location

`web.go` — session management map.

## Fix

Protect with `sync.RWMutex`. Add `-race` test.

## Status

- [ ] TODO
