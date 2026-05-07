# Issue #025: No Rate Limiting on Login Endpoint

**Severity:** MEDIUM

## Issue

The login endpoint has no rate limiting or lockout. An attacker on the local network can brute-force the password indefinitely.

## Location

`web.go` — login handler.

## Fix

Add exponential backoff or a fixed delay after failed attempts (e.g., 1s after 3 failures, lockout after 10). Add a test.

## Status

- [ ] TODO
