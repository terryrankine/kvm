# Issue #034: Macro Payload Length Not Validated

**Severity:** MEDIUM

## Issue

Keyboard macro payloads are accepted without a length limit. A very long macro can cause the HID write loop to block for an extended period, locking out other input.

## Location

`web.go` or `hid.go` — macro execution handler.

## Fix

Limit macro payloads to a reasonable maximum (e.g., 1000 keystrokes). Return an error for oversized payloads. Add a test.

## Status

- [x] ALREADY FIXED — `config.go` defines `MaxStepsPerMacro = 10` and `KeyboardMacro.Validate()` enforces it (`len(m.Steps) > MaxStepsPerMacro` → error). All macro save paths call `macro.Validate()` before accepting the payload. No code change required.
