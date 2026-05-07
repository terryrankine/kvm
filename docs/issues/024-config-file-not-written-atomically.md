# Issue #024: Config File Not Written Atomically

**Severity:** MEDIUM

## Issue

Config is written by truncating and rewriting the existing file. A crash mid-write produces a zero-length or partially-written config, bricking the device configuration.

## Location

`config.go` — `saveConfig()` or equivalent.

## Fix

Write to a `.tmp` file then `os.Rename()`. Add a test that kills the write process mid-way and verifies the original config is intact.

## Status

- [ ] TODO
