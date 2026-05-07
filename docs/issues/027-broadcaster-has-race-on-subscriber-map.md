# Issue #027: Broadcaster Has Race on Subscriber Map

**Severity:** MEDIUM

## Issue

The event broadcaster's subscriber map is modified (add/remove) while potentially being iterated over in the broadcast goroutine, causing a map concurrent read/write panic.

## Location

`broadcast.go` or equivalent.

## Fix

Protect the subscriber map with a `sync.RWMutex` or use a channel-based design. Add `-race` test.

## Status

- [ ] TODO
