# Issue #010: WebSocket Origin Check Disabled

**Severity:** HIGH

## Issue

The WebSocket upgrade handler disables origin checking (`CheckOrigin: func(r *http.Request) bool { return true }`). This allows any origin to establish a WebSocket connection, enabling CSRF-style attacks via a malicious webpage visited by a user on the same network.

## Location

`web.go` — `websocket.Upgrader` configuration.

## Fix

Validate the `Origin` header against the device hostname and `localhost`. Reject connections from unknown origins with HTTP 403. Add a test with a spoofed Origin header.

## Status

- [x] DONE — Replaced `InsecureSkipVerify: true` with `OriginPatterns: []string{c.Request.Host}` in `web.go` `handleLocalWebRTCSignal`. Only same-origin connections (matching the server's Host header) are now accepted; cross-origin requests from other sites are rejected.
