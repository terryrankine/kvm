# Issue #028: chmod Return Value Not Checked

**Severity:** MEDIUM

## Issue

`os.Chmod()` calls on HID device files or config directories ignore the error return. On a read-only filesystem or permission error this silently fails, leading to confusing downstream failures.

## Location

Various — `chmod` calls in `hid.go`, `config.go`.

## Fix

Check and log `chmod` errors. Add a test with a read-only target.

## Status

- [x] ALREADY FIXED — All `os.Chmod()` calls in the codebase (`native.go`, `native_audio.go`, `native_display.go`, `native_vpn.go`, `ota.go`, `ota_offline.go`) already check and handle the error return. No code change required.
