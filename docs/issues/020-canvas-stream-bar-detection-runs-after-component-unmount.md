# Issue #020: Canvas Stream Bar Detection Runs After Component Unmount

**Severity:** HIGH

## Issue

`detectStreamBars` is called via `setTimeout(..., 1000)` after video starts playing. If the component unmounts before the 1-second timeout fires, the callback still runs and calls `setStreamContentBounds` on an unmounted store, which may cause React state update warnings or stale closures.

## Location

`ui/src/layout/core/desktop/hooks/useVideoStream.ts` — `markAsPlaying` / `detectStreamBars`.

## Fix

Store the timeout ID and clear it in the `useEffect` cleanup. Add a test that unmounts the component before 1s and asserts no store updates occur.

## Status

- [ ] TODO
