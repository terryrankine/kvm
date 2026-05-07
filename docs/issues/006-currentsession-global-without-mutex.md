# Issue #006: currentSession Global Without Mutex

**Severity:** CRITICAL

## Issue

`currentSession` is a package-level variable written by login/logout handlers and read by the auth middleware. No mutex protects it. Under concurrent requests (browser pre-fetch, WebSocket upgrade, API calls) this is a data race.

## Location

`web.go` — `var currentSession *Session`.

## Fix

Replace bare global with a `sync.RWMutex`-protected accessor pair `getSession()` / `setSession()`. Add `-race` to CI test flags.

## Status

- [x] DONE — Added `currentSessionMu sync.RWMutex` and `getSession()`/`setSession()` accessors in `web.go`. All direct reads replaced with `getSession()` and writes with `setSession()` across `web.go`, `native.go`, `audio.go`, `cloud.go`, `jsonrpc.go`, `network.go`, `ota.go`, `usb.go`, `video.go`, `webrtc.go`.
