# Issue #029: EDID Read Errors Silently Dropped

**Severity:** MEDIUM

## Issue

EDID read failures are swallowed with `_ =` or empty error handlers. The UI never learns that EDID is unavailable, potentially displaying incorrect resolution options.

## Location

`video.go` or `edid.go`.

## Fix

Log EDID errors at WARN and surface a `edidAvailable: false` flag in the device status API. Add a test.

## Status

- [ ] TODO
