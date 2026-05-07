# Issue #003: bcrypt Cost Too Low

**Severity:** CRITICAL

## Issue

Password hashing uses bcrypt with cost factor 10 (library default). For an embedded device that is rarely used for login, cost 10 is acceptable, but the value is not explicitly set and may silently change with library upgrades. More critically, the hash is compared in a timing-unsafe way in at least one code path.

## Location

`auth.go` (or equivalent) — `bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)`.

## Fix

Pin cost to `bcrypt.DefaultCost` (explicit constant). Use `bcrypt.CompareHashAndPassword` (already constant-time) everywhere — remove any `==` string comparisons on hashes. Add a test asserting the stored hash round-trips correctly via `CompareHashAndPassword`.

## Status

- [x] ALREADY ACCEPTABLE — `web.go` uses `bcrypt.GenerateFromPassword(..., bcrypt.DefaultCost)` explicitly (cost 10) at all hash generation sites. All comparisons use `bcrypt.CompareHashAndPassword` which is already constant-time. No `==` comparisons on hash strings. Cost 10 is appropriate for a single-user embedded device that authenticates rarely. No code change required.
