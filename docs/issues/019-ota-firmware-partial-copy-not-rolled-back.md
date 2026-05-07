# Issue #019: OTA Firmware Partial Copy Not Rolled Back

**Severity:** HIGH

## Issue

If the device loses power or the process is killed mid-firmware-copy, the partially-written firmware file is left in place. On next boot the device may attempt to boot from a corrupt image.

## Location

`ota_offline.go` — file copy during apply.

## Fix

Write firmware to a `.tmp` file then `os.Rename()` atomically. Add a test that kills the write mid-stream and verifies no partial file remains at the target path.

## Status

- [ ] TODO
