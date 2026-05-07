# Issue #016: Goroutine Leak in HTTP Stream Handler

**Severity:** HIGH

## Issue

The HTTP video stream handler (`/video/stream`) spawns goroutines that may not be cancelled when the client disconnects, leading to resource leaks.

## Location

`web.go` or `video.go` — `/video/stream` handler.

## Fix

Pass `r.Context()` to the streaming goroutine and check for cancellation. Add a test.

## Status

- [x] NO LEAK — `handleVideoStream` uses `c.Request.Context()` in the select; client disconnect triggers `ctx.Done()` and the function returns, at which point `defer videoBroadcaster.Unsubscribe(id)` fires. No goroutine spawned. No code change required.
