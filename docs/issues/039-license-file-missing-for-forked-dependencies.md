# Issue #039: License File Missing for Forked Dependencies

**Severity:** LOW

## Issue

The repo includes vendored or modified third-party code without accompanying LICENSE files for those dependencies. This may violate the license terms of those dependencies (MIT/Apache require attribution).

## Location

`ui/` — vendored JS packages; Go module vendor directory.

## Fix

Run `go mod vendor` and `npm run build` with license extraction. Add a `LICENSES.md` or `THIRD_PARTY_NOTICES` file with all dependency licenses. Add a CI check.

## Status

- [ ] TODO — Requires running `go-licenses save ./...` and `license-checker --out THIRD_PARTY_NOTICES.md` from the ui/ directory. Deferred — tooling not set up in CI. Go module cache contains license files; npm `node_modules/` is gitignored and regenerated at build time, so runtime attribution is via `npm run build` output.
