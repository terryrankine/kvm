# Issue #008: OTA Handler Missing Panic Recovery

**Severity:** CRITICAL

## Issue

`ota_offline.go` upload/apply handlers have no `recover()`. A panic during firmware extraction (malformed zip, out-of-memory) will crash the entire `kvm_app` process, requiring a manual power cycle to recover the device.

## Location

`ota_offline.go` — HTTP handlers for OTA upload and apply.

## Fix

Add a deferred `recover()` at the top of each handler that returns HTTP 500 with a safe error message. Add a test with a deliberately malformed payload and assert the server returns 500 and stays alive.

## Status

- [x] DONE — Added `defer recover()` to both `handleOfflineUpdateUpload` and `handleOfflineUpdateApply` in `ota_offline.go`. Panics are logged at ERROR level and return HTTP 500 with a safe message instead of crashing the process.
