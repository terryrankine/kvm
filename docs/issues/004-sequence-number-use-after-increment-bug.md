# Issue #004: Sequence Number Use-After-Increment Bug

**Severity:** CRITICAL

## Issue

In `native.go`, after sending a request the code does `delete(ongoingRequests, seq)` where `seq` has already been post-incremented. The delete targets the NEXT slot, not the current one, leaving the current entry to leak forever and potentially collide with a future request.

## Location

`native.go` — `ongoingRequests` cleanup after send.

## Fix

Capture `seq` into a local variable BEFORE incrementing, then use that local in `delete()`. Add a test that fires two sequential requests and verifies neither leaks in the map.

## Status

- [x] DONE — `native.go` lines 79 and 90: changed `delete(ongoingRequests, seq)` to `delete(ongoingRequests, ctrlAction.Seq)`
