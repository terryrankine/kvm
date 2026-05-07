# Issue #032: WebSocket Connection Not Cleanly Closed on Server Shutdown

**Severity:** MEDIUM

## Issue

On graceful shutdown the server does not send WebSocket close frames to connected clients. Clients detect the drop as an error and show a disconnection error rather than a clean reconnect prompt.

## Location

`web.go` — server shutdown handler.

## Fix

On shutdown, iterate active WebSocket connections and send `CloseNormalClosure` frames before `httpServer.Shutdown()`. Add a test.

## Status

- [ ] TODO
