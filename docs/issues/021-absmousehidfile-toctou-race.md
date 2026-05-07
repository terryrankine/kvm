# Issue #021: absMouseHidFile TOCTOU Race

**Severity:** HIGH

## Issue

The absolute mouse HID handler checks for file existence then opens it in two separate syscalls (check-then-act). Between the check and the open the file could be removed or replaced, causing writes to go to an unexpected file descriptor.

## Location

`hid.go` — absolute mouse device file open.

## Fix

Open the file directly and handle the error rather than pre-checking existence. Add a test.

## Status

- [ ] TODO
