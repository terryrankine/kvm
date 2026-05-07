# Issue #017: USB Gadget Callbacks Race

**Severity:** HIGH

## Issue

USB gadget state change callbacks are invoked from a system event goroutine while the HID write path also accesses gadget state. No synchronization exists between them.

## Location

`internal/usbgadget/` — state change callbacks.

## Fix

Protect gadget state reads/writes with a mutex. Add `-race` test.

## Status

- [x] DONE — Added `callbackLock sync.RWMutex` to `UsbGadget` struct. All `SetOn*` methods write under `callbackLock.Lock()`. All callback invocation sites in `hid_keyboard.go`, `hid_mouse_absolute.go`, `hid_mouse_relative.go` capture the pointer under `callbackLock.RLock()` before checking nil and invoking.
