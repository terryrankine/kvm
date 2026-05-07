# Issue #033: Keyboard Capture Mode Has No Visual Confirmation

**Severity:** MEDIUM

## Issue

When the browser captures keyboard input (pointer lock or focus mode) there is no persistent visible indicator. Users may type into unintended applications thinking the KVM is not capturing.

## Location

`ui/src/layout/core/desktop/` — keyboard capture state display.

## Fix

Show a persistent banner or border highlight when keyboard capture is active. Add a Playwright test.

## Status

- [x] ALREADY FIXED — `BottomBarPC.tsx` shows "KB Capture" button in blue (`rgba(22,152,217,1)`) when active, with "Active" or "Limited" sub-label. A toast notification fires on toggle. Visual confirmation is already present in the bottom bar. No code change required.
