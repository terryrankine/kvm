# Issue #017: USB Gadget Callbacks Race

**Severity:** HIGH

## Issue

USB gadget state change callbacks are invoked from a system event goroutine while the HID write path also accesses gadget state. No synchronization exists between them.

## Location

`internal/usbgadget/` — state change callbacks.

## Fix

Protect gadget state reads/writes with a mutex. Add `-race` test.

## Status

- [ ] TODO
