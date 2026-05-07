# Issue #006: currentSession Global Without Mutex

**Severity:** CRITICAL

## Issue

`currentSession` is a package-level variable written by login/logout handlers and read by the auth middleware. No mutex protects it. Under concurrent requests (browser pre-fetch, WebSocket upgrade, API calls) this is a data race.

## Location

`web.go` — `var currentSession *Session`.

## Fix

Replace bare global with a `sync.RWMutex`-protected accessor pair `getSession()` / `setSession()`. Add `-race` to CI test flags.

## Status

- [ ] TODO
