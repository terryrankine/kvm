# Issue #009: Settings Persisted to localStorage (XSS Risk)

**Severity:** CRITICAL

## Issue

Sensitive UI settings (possibly including session tokens or device credentials visible in the UI) are written to `localStorage`. Any XSS vulnerability in the app can exfiltrate these values. `localStorage` also persists across sessions and is readable by any same-origin script.

## Location

`ui/src/hooks/stores.ts` — Zustand persist middleware writing to `localStorage`.

## Fix

Audit what is persisted. Move security-sensitive values (tokens, credentials) to `sessionStorage` or in-memory only. Add a test that mounts the store and asserts sensitive keys are not present in `localStorage`.

## Status

- [ ] TODO
