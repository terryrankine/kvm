# Issue #007: Keyboard HID Package-Level Lock

**Severity:** CRITICAL

## Issue

`hid_keyboard.go` uses a package-level mutex or global state shared across all HID keyboard instances. If multiple sessions or goroutines use the keyboard simultaneously the lock becomes a bottleneck and may deadlock if a caller panics while holding it.

## Location

`internal/usbgadget/hid_keyboard.go`.

## Fix

Move the lock inside the struct so each instance has its own lock. Add a test that creates two instances and writes concurrently to assert no deadlock.

## Status

- [ ] TODO
