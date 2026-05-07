# Issue #022: No CSRF Protection on State-Changing Endpoints

**Severity:** MEDIUM

## Issue

Endpoints that change device state (reboot, set password, OTA apply) have no CSRF token requirement. A malicious page visited by a logged-in user can trigger these actions via a cross-origin form POST.

## Location

`web.go` — state-changing API handlers.

## Fix

Require a `X-Requested-With: XMLHttpRequest` header (simple CSRF barrier) or implement proper CSRF tokens. Add a test.

## Status

- [x] DONE — Added `c.SetSameSite(http.SameSiteStrictMode)` before every `SetCookie("authToken", ...)` in `web.go`. `SameSite=Strict` prevents the browser from sending the auth cookie on any cross-site request (navigation, form POST, fetch), blocking CSRF attacks without requiring a token. WebSocket upgrade is also origin-checked (issue #010).
