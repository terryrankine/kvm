# Issue #001: setConfigRaw Auth Bypass

**Severity:** CRITICAL

## Issue

`setConfigRaw` endpoint (or equivalent config write path) does not require authentication. Any unauthenticated caller can overwrite device configuration including the auth token, effectively locking out the legitimate owner or gaining persistent access.

## Location

`web.go` — config update handler registered without auth middleware.

## Fix

Register all `/api/` config mutation endpoints behind `authMiddleware()`. Add integration test that POSTs to `/api/config` without a session cookie and asserts HTTP 401.

## Status

- [ ] TODO
