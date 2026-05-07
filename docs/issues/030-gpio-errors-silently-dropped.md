# Issue #030: GPIO Errors Silently Dropped

**Severity:** MEDIUM

## Issue

GPIO operations (ATX power button, reset button) ignore errors from the sysfs write. A failed button press gives no feedback to the user.

## Location

`atx.go` or `gpio.go`.

## Fix

Return and surface GPIO errors to the JSON-RPC caller. Add a test with a mocked GPIO path.

## Status

- [ ] TODO
