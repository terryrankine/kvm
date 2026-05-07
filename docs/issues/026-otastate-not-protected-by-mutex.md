# Issue #026: otaState Not Protected by Mutex

**Severity:** MEDIUM

## Issue

`otaState` (OTA progress tracking struct) is read and written from HTTP handlers and background goroutines without a lock.

## Location

`ota_offline.go` — `otaState` struct.

## Fix

Add a `sync.Mutex` to `otaState`. Add `-race` test.

## Status

- [x] DONE — Added `otaStateMu sync.RWMutex` in `ota.go`. Protected `IsUpdatePending()` with RLock, `triggerOTAStateUpdate()` snapshot with RLock, and the check-then-set `Updating` transitions in both `TryUpdate()` and `applyOfflineUpdate()` with write Lock.
