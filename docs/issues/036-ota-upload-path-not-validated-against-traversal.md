# Issue #036: OTA Upload Path Not Validated Against Traversal

**Severity:** MEDIUM

## Issue

The OTA upload endpoint writes the uploaded file to a path derived from the request without fully sanitizing it. A crafted filename containing `../` could write outside the intended directory.

## Location

`ota_offline.go` — upload handler filename handling.

## Fix

Use `filepath.Base()` on the uploaded filename and join it to a fixed base directory. Assert the resulting path is within the expected directory. Add a test with a traversal payload.

## Status

- [ ] TODO
