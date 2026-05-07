# Issue #016: Goroutine Leak in HTTP Stream Handler

**Severity:** HIGH

## Issue

The HTTP video stream handler (`/video/stream`) spawns goroutines that may not be cancelled when the client disconnects, leading to resource leaks.

## Location

`web.go` or `video.go` — `/video/stream` handler.

## Fix

Pass `r.Context()` to the streaming goroutine and check for cancellation. Add a test.

## Status

- [ ] TODO
