# Issue #012: HID Report Missing Input Validation

**Severity:** HIGH

## Issue

Mouse and keyboard HID reports accept raw values from the JSON-RPC call without range-checking. Out-of-range values written to the HID gadget device file can corrupt the USB descriptor or cause kernel driver errors.

## Location

`hid.go` or equivalent RPC handlers — `absMouseReport`, `relMouseReport`, `keyboardReport`.

## Fix

Clamp or reject out-of-range values before writing to the gadget device. Add tests for boundary values (0, 32767, -1, 32768 for abs mouse).

## Status

- [ ] TODO
