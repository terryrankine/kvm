# Issue #033: Keyboard Capture Mode Has No Visual Confirmation

**Severity:** MEDIUM

## Issue

When the browser captures keyboard input (pointer lock or focus mode) there is no persistent visible indicator. Users may type into unintended applications thinking the KVM is not capturing.

## Location

`ui/src/layout/core/desktop/` — keyboard capture state display.

## Fix

Show a persistent banner or border highlight when keyboard capture is active. Add a Playwright test.

## Status

- [ ] TODO
