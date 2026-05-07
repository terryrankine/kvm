# Issue #018: WriteSample Errors Silently Dropped

**Severity:** HIGH

## Issue

`WriteSample` calls (video/audio frame writing to WebRTC track) discard errors with `_ =`. A write failure means frames are silently dropped rather than triggering reconnection or surfacing an error to the user.

## Location

`native.go` or `webrtc.go` — `track.WriteSample(...)`.

## Fix

Log write errors at WARN level, increment a counter, and trigger peer connection renegotiation after N consecutive failures. Add a test.

## Status

- [ ] TODO
