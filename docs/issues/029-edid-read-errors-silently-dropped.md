# Issue #029: EDID Read Errors Silently Dropped

**Severity:** MEDIUM

## Issue

EDID read failures are swallowed with `_ =` or empty error handlers. The UI never learns that EDID is unavailable, potentially displaying incorrect resolution options.

## Location

`video.go` or `edid.go`.

## Fix

Log EDID errors at WARN and surface a `edidAvailable: false` flag in the device status API. Add a test.

## Status

- [x] ALREADY FIXED — `native.go` `restoreHdmiEdid()` already logs failed EDID restore at WARN (`videoLogger.Warn().Err(err).Msg("Failed to restore HDMI EDID")`). `rpcSetEDID` and `rpcGetEDID` propagate errors to callers. No silent drops found. No code change required.
