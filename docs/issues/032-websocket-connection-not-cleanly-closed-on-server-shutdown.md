# Issue #032: WebSocket Connection Not Cleanly Closed on Server Shutdown

**Severity:** MEDIUM

## Issue

On graceful shutdown the server does not send WebSocket close frames to connected clients. Clients detect the drop as an error and show a disconnection error rather than a clean reconnect prompt.

## Location

`web.go` — server shutdown handler.

## Fix

On shutdown, iterate active WebSocket connections and send `CloseNormalClosure` frames before `httpServer.Shutdown()`. Add a test.

## Status

- [ ] TODO — `main.go` exits on SIGTERM without calling `httpServer.Shutdown()` or sending WS close frames. Requires: exposing the `http.Server` from `startWebServer`, tracking active WS connections in a registry, sending `StatusNormalClosure` on shutdown signal, then calling `server.Shutdown(ctx)`. Deferred — significant refactor.
